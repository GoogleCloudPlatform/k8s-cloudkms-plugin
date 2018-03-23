/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import (
	"fmt"
	"net/http"

	"io"

	"os"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	cloudkms "google.golang.org/api/cloudkms/v1"
	gcfg "gopkg.in/gcfg.v1"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
)

type TokenConfig struct {
	Global struct {
		TokenURL  string `gcfg:"token-url"`
		TokenBody string `gcfg:"token-body"`
	}
}

func newHTTPClient(pathToGCEConf string) (*http.Client, error) {
	if pathToGCEConf == "" {
		ctx := context.Background()
		client, err := google.DefaultClient(ctx, cloudkms.CloudPlatformScope)
		if err != nil {
			return nil, fmt.Errorf("failed to instantiate cloud sdk client: %v", err)
		}
		return client, nil
	}

	r, err := os.Open(pathToGCEConf)
	if err != nil {
		return nil, fmt.Errorf("failed to open GCE Config: %s", pathToGCEConf)
	}

	c, err := readConfig(r)
	if err != nil {
		return nil, err
	}

	a := gce.NewAltTokenSource(c.Global.TokenURL, c.Global.TokenBody)
	// TODO: Do I need to call a.Token to get access token?

	return oauth2.NewClient(oauth2.NoContext, a), nil
}

func readConfig(reader io.Reader) (*TokenConfig, error) {
	cfg := &TokenConfig{}
	if err := gcfg.FatalOnly(gcfg.ReadInto(cfg, reader)); err != nil {
		glog.Errorf("Couldn't read GCE Config: %v", err)
		return nil, err
	}
	return cfg, nil
}
