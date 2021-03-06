//
// Copyright 2020 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package shield

import (
	"strings"

	rsigapi "github.com/IBM/integrity-enforcer/shield/pkg/apis/resourcesignature/v1alpha1"
	rspapi "github.com/IBM/integrity-enforcer/shield/pkg/apis/resourcesigningprofile/v1alpha1"
	sigconfapi "github.com/IBM/integrity-enforcer/shield/pkg/apis/signerconfig/v1alpha1"

	common "github.com/IBM/integrity-enforcer/shield/pkg/common"
	config "github.com/IBM/integrity-enforcer/shield/pkg/shield/config"
)

// check if request is inScope or not
func inScopeCheck(reqc *common.ReqContext, config *config.ShieldConfig, data *RunData, ctx *CheckContext) *DecisionResult {
	reqNamespace := getRequestNamespaceFromReqContext(reqc)

	// check if reqNamespace matches ShieldConfig.MonitoringNamespace and check if any RSP is targeting the namespace
	// this check is done only for Namespaced request, and skip this for Cluster-scope request
	if reqNamespace != "" && !checkIfInScopeNamespace(reqNamespace, config) && !checkIfProfileTargetNamespace(reqNamespace, config.Namespace, data) {
		msg := "this namespace is not monitored"
		ctx.Allow = true
		ctx.ReasonCode = common.REASON_INTERNAL
		ctx.Message = msg
		return &DecisionResult{
			Type:       common.DecisionAllow,
			ReasonCode: common.REASON_INTERNAL,
			Message:    msg,
		}
	}

	if checkIfDryRunAdmission(reqc) {
		msg := "request is dry run"
		ctx.Allow = true
		ctx.ReasonCode = common.REASON_INTERNAL
		ctx.Message = msg
		return &DecisionResult{
			Type:       common.DecisionAllow,
			ReasonCode: common.REASON_INTERNAL,
			Message:    msg,
		}
	}

	if checkIfUnprocessedInIShield(reqc, config) {
		msg := "request is not processed by IShield"
		ctx.Allow = true
		ctx.ReasonCode = common.REASON_INTERNAL
		ctx.Message = msg
		return &DecisionResult{
			Type:       common.DecisionAllow,
			ReasonCode: common.REASON_INTERNAL,
			Message:    msg,
		}
	}

	return undeterminedDescision()
}

func formatCheck(reqc *common.ReqContext, config *config.ShieldConfig, data *RunData, ctx *CheckContext) *DecisionResult {
	if ok, msg := ValidateResource(reqc, config.Namespace); !ok {
		ctx.Allow = false
		ctx.ReasonCode = common.REASON_VALIDATION_FAIL
		ctx.Message = msg
		return &DecisionResult{
			Type:       common.DecisionDeny,
			ReasonCode: common.REASON_VALIDATION_FAIL,
			Message:    msg,
		}
	}
	return undeterminedDescision()
}

func iShieldResourceCheck(reqc *common.ReqContext, config *config.ShieldConfig, data *RunData, ctx *CheckContext) *DecisionResult {
	reqRef := reqc.ResourceRef()
	iShieldOperatorResource := config.IShieldResourceCondition.IsOperatorResource(reqRef)
	iShieldServerResource := config.IShieldResourceCondition.IsServerResource(reqRef)

	if !iShieldOperatorResource && !iShieldServerResource {
		return undeterminedDescision()
	} else {
		ctx.IShieldResource = true
	}

	iShieldAdmin := checkIfIShieldAdminRequest(reqc, config)
	iShieldServer := checkIfIShieldServerRequest(reqc, config)
	iShieldOperator := checkIfIShieldOperatorRequest(reqc, config)

	if (iShieldOperatorResource && iShieldAdmin) || (iShieldServerResource && (iShieldOperator || iShieldServer)) {
		ctx.Allow = true
		ctx.Verified = true
		ctx.ReasonCode = common.REASON_ISHIELD_ADMIN
		ctx.Message = common.ReasonCodeMap[common.REASON_ISHIELD_ADMIN].Message
		return &DecisionResult{
			Type:       common.DecisionAllow,
			Verified:   true,
			ReasonCode: common.REASON_ISHIELD_ADMIN,
			Message:    common.ReasonCodeMap[common.REASON_ISHIELD_ADMIN].Message,
		}
	} else {
		ctx.Allow = false
		ctx.Verified = false
		ctx.ReasonCode = common.REASON_BLOCK_ISHIELD_RESOURCE_OPERATION
		ctx.Message = common.ReasonCodeMap[common.REASON_BLOCK_ISHIELD_RESOURCE_OPERATION].Message
		return &DecisionResult{
			Type:       common.DecisionDeny,
			Verified:   false,
			ReasonCode: common.REASON_BLOCK_ISHIELD_RESOURCE_OPERATION,
			Message:    common.ReasonCodeMap[common.REASON_BLOCK_ISHIELD_RESOURCE_OPERATION].Message,
		}
	}
}

func deleteCheck(reqc *common.ReqContext, config *config.ShieldConfig, data *RunData, ctx *CheckContext) *DecisionResult {
	if reqc.IsDeleteRequest() {
		ctx.Allow = true
		ctx.Verified = true
		ctx.ReasonCode = common.REASON_SKIP_DELETE
		ctx.Message = common.ReasonCodeMap[common.REASON_SKIP_DELETE].Message
		return &DecisionResult{
			Type:       common.DecisionAllow,
			Verified:   true,
			ReasonCode: common.REASON_SKIP_DELETE,
			Message:    common.ReasonCodeMap[common.REASON_SKIP_DELETE].Message,
		}
	}
	return undeterminedDescision()
}

func protectedCheck(reqc *common.ReqContext, config *config.ShieldConfig, data *RunData, ctx *CheckContext) (*DecisionResult, []rspapi.ResourceSigningProfile) {
	reqFields := reqc.Map()
	ruleTable := data.GetRuleTable(config.Namespace)
	if ruleTable == nil {
		ctx.Allow = true
		ctx.Verified = true
		ctx.Protected = false
		ctx.ReasonCode = common.REASON_NOT_PROTECTED
		ctx.Message = common.ReasonCodeMap[common.REASON_NOT_PROTECTED].Message
		return &DecisionResult{
			Type:       common.DecisionAllow,
			ReasonCode: common.REASON_NOT_PROTECTED,
			Message:    common.ReasonCodeMap[common.REASON_NOT_PROTECTED].Message,
		}, nil
	}
	protected, ignoreMatched, matchedProfiles := ruleTable.CheckIfProtected(reqFields)
	if !protected {
		ctx.Allow = true
		ctx.Verified = true
		ctx.Protected = false
		if ignoreMatched {
			ctx.ReasonCode = common.REASON_IGNORE_RULE_MATCHED
			ctx.Message = common.ReasonCodeMap[common.REASON_IGNORE_RULE_MATCHED].Message
			return &DecisionResult{
				Type:       common.DecisionAllow,
				ReasonCode: common.REASON_IGNORE_RULE_MATCHED,
				Message:    common.ReasonCodeMap[common.REASON_IGNORE_RULE_MATCHED].Message,
			}, nil
		} else {
			ctx.ReasonCode = common.REASON_NOT_PROTECTED
			ctx.Message = common.ReasonCodeMap[common.REASON_NOT_PROTECTED].Message
			return &DecisionResult{
				Type:       common.DecisionAllow,
				ReasonCode: common.REASON_NOT_PROTECTED,
				Message:    common.ReasonCodeMap[common.REASON_NOT_PROTECTED].Message,
			}, nil
		}
	} else {
		ctx.Protected = true
	}
	return undeterminedDescision(), matchedProfiles
}

func resourceSigningProfileCheck(singleProfile rspapi.ResourceSigningProfile, reqc *common.ReqContext, config *config.ShieldConfig, data *RunData, ctx *CheckContext) *DecisionResult {
	var allowed bool
	var evalMessage string
	var evalReason int
	var sigResult *common.SignatureEvalResult
	var mutResult *common.MutationEvalResult

	sigConf := data.GetSignerConfig()
	rsigList := data.GetResSigList(reqc)

	allowed, evalReason, evalMessage, sigResult, mutResult = singleProfileCheck(singleProfile, reqc, config, sigConf, rsigList)

	ctx.Allow = allowed
	ctx.ReasonCode = evalReason
	ctx.Message = evalMessage
	if sigResult != nil {
		ctx.SignatureEvalResult = sigResult
	}
	if mutResult != nil {
		ctx.MutationEvalResult = mutResult
	}

	if allowed {
		ctx.Verified = true
		return &DecisionResult{
			Type:       common.DecisionAllow,
			Verified:   true,
			ReasonCode: evalReason,
			Message:    evalMessage,
		}
	} else {
		return &DecisionResult{
			Type:       common.DecisionDeny,
			ReasonCode: evalReason,
			Message:    evalMessage,
			denyRSP:    &singleProfile,
		}
	}
}

func singleProfileCheck(singleProfile rspapi.ResourceSigningProfile, reqc *common.ReqContext, config *config.ShieldConfig, sigConfRes *sigconfapi.SignerConfig, rsigList *rsigapi.ResourceSignatureList) (bool, int, string, *common.SignatureEvalResult, *common.MutationEvalResult) {
	var sigResult *common.SignatureEvalResult
	var mutResult *common.MutationEvalResult
	var err error
	if reqc.IsUpdateRequest() {
		mutResult, err = NewMutationChecker().Eval(reqc, singleProfile)
		if err != nil {
			return false, common.REASON_ERROR, err.Error(), nil, mutResult
		}
		if mutResult.Checked && !mutResult.IsMutated {
			return true, common.REASON_NO_MUTATION, common.ReasonCodeMap[common.REASON_NO_MUTATION].Message, nil, mutResult
		}
	}

	signerConfig := sigConfRes.Spec.Config
	plugins := config.GetEnabledPlugins()
	evaluator, err := NewSignatureEvaluator(config, signerConfig, plugins)
	if err != nil {
		return false, common.REASON_ERROR, err.Error(), nil, mutResult
	}
	sigResult, err = evaluator.Eval(reqc, rsigList, singleProfile)
	if err != nil {
		return false, common.REASON_ERROR, err.Error(), sigResult, mutResult
	}
	if sigResult.Checked && sigResult.Allow {
		return true, common.REASON_VALID_SIG, common.ReasonCodeMap[common.REASON_VALID_SIG].Message, sigResult, mutResult
	}

	var reasonCode int
	var message string
	if sigResult.Error != nil {
		message = sigResult.Error.MakeMessage()
		if strings.HasPrefix(message, common.ReasonCodeMap[common.REASON_INVALID_SIG].Message) {
			reasonCode = common.REASON_INVALID_SIG
		} else if strings.HasPrefix(message, common.ReasonCodeMap[common.REASON_NO_VALID_KEYRING].Message) {
			reasonCode = common.REASON_NO_VALID_KEYRING
		} else if strings.HasPrefix(message, common.ReasonCodeMap[common.REASON_NO_MATCH_SIGNER_CONFIG].Message) {
			reasonCode = common.REASON_NO_MATCH_SIGNER_CONFIG
		} else if message == common.ReasonCodeMap[common.REASON_NO_SIG].Message {
			reasonCode = common.REASON_NO_SIG
		} else {
			reasonCode = common.REASON_ERROR
		}
	}
	return false, reasonCode, message, sigResult, mutResult
}
