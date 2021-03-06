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
	"context"
	"encoding/json"
	"time"

	cache "github.com/IBM/integrity-enforcer/shield/pkg/util/cache"
	"github.com/IBM/integrity-enforcer/shield/pkg/util/kubeutil"
	logger "github.com/IBM/integrity-enforcer/shield/pkg/util/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/core/v1"
	v1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Namespace

type NamespaceLoader struct {
	interval time.Duration
	Client   *v1client.CoreV1Client
	Data     []v1.Namespace
}

func NewNamespaceLoader() *NamespaceLoader {
	interval := time.Second * 30
	config, _ := kubeutil.GetKubeConfig()
	client, _ := v1client.NewForConfig(config)

	return &NamespaceLoader{
		interval: interval,
		Client:   client,
	}
}

func (self *NamespaceLoader) GetData(doK8sApiCall bool) ([]v1.Namespace, bool) {
	reloaded := false
	if len(self.Data) == 0 {
		reloaded = self.Load(doK8sApiCall)
	}
	return self.Data, reloaded
}

func (self *NamespaceLoader) Load(doK8sApiCall bool) bool {
	var err error
	var list1 *v1.NamespaceList
	var keyName string
	reloaded := false

	keyName = "NamespaceLoader/list"
	if cached := cache.GetString(keyName); cached == "" && doK8sApiCall {
		list1, err = self.Client.Namespaces().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logger.Error("failed to get Namespace:", err)
			return false
		}
		reloaded = true
		logger.Debug("Namespace reloaded.")
		if len(list1.Items) > 0 {
			tmp, _ := json.Marshal(list1)
			cache.SetString(keyName, string(tmp), &(self.interval))
		}
	} else if cached != "" {
		err = json.Unmarshal([]byte(cached), &list1)
		if err != nil {
			logger.Error("failed to Unmarshal cached Namespace:", err)
			return false
		}
	}

	data := []v1.Namespace{}
	if list1 != nil && len(list1.Items) > 0 {
		data = list1.Items
	}
	self.Data = data
	return reloaded
}

func (self *NamespaceLoader) ClearCache() {
	cache.Unset("NamespaceLoader/list")
}
