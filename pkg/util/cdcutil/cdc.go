// Copyright 2024 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cdcutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/log"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

const (
	// CDCPrefix is the prefix of CDC information in etcd.
	CDCPrefix = "/tidb/cdc/"
	// ChangefeedPath is the path of changefeed information in etcd.
	ChangefeedPath = "/changefeed/info/"
	// CDCPrefixV61 is the prefix of CDC information in etcd.
	CDCPrefixV61 = "/tidb/cdc/changefeed/info/"
)

// CDCNameSet saves CDC changefeed's information.
// nameSet maps `cluster/namespace` to `changefeed`s
type CDCNameSet struct {
	nameSet map[string][]string
}

// Empty that the nameSet is empty means no changefeed exists.
func (s *CDCNameSet) Empty() bool {
	return len(s.nameSet) == 0
}

// MessageToUser convert the map `nameSet` to a readable message to user.
func (s *CDCNameSet) MessageToUser() string {
	var changefeedMsgBuf strings.Builder
	changefeedMsgBuf.WriteString("found CDC changefeed(s): ")
	for clusterID, captureIDs := range s.nameSet {
		changefeedMsgBuf.WriteString("cluster/namespace: ")
		changefeedMsgBuf.WriteString(clusterID)
		changefeedMsgBuf.WriteString(" changefeed(s): ")
		changefeedMsgBuf.WriteString(fmt.Sprintf("%v", captureIDs))
		changefeedMsgBuf.WriteString(", ")
	}
	return changefeedMsgBuf.String()
}

// GetCDCChangefeedNameSet gets CDC changefeed information and wraps them to a map
// for CDC >= v6.2, the etcd key format is /tidb/cdc/<clusterID>/<namespace>/changefeed/info/<changefeedID>
// for CDC <= v6.1, the etcd key format is /tidb/cdc/changefeed/info/<changefeedID>
func GetCDCChangefeedNameSet(ctx context.Context, cli *clientv3.Client) (*CDCNameSet, error) {
	nameSet := make(map[string][]string, 1)
	// check etcd KV of CDC >= v6.2
	resp, err := cli.Get(ctx, CDCPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, errors.Trace(err)
	}

	// cluster id should be valid in
	// https://github.com/pingcap/tiflow/blob/ca69c33948bea082aff9f4c0a357ace735b494ed/pkg/config/server_config.go#L218
	reg, err := regexp.Compile("^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$")
	if err != nil {
		log.L().Warn("failed to parse cluster id, skip it", zap.Error(err))
		reg = nil
	}

	for _, kv := range resp.Kvs {
		// example: /tidb/cdc/<clusterID>/<namespace>/changefeed/info/<changefeedID>
		k := kv.Key[len(CDCPrefix):]
		clusterAndNamespace, changefeedID, found := bytes.Cut(k, []byte(ChangefeedPath))
		if !found {
			continue
		}
		// example: clusterAndNamespace normally is <clusterID>/<namespace>
		// but in migration scenario it become __backup__. we need handle it
		// see https://github.com/pingcap/tiflow/issues/9807
		clusterID, _, found := bytes.Cut(clusterAndNamespace, []byte(`/`))
		if !found {
			// ignore __backup__ or other formats
			continue
		}

		if reg != nil {
			matched := reg.Match(clusterID)
			if !matched {
				continue
			}
			if !isActiveCDCChangefeed(kv.Value) {
				continue
			}
		}

		nameSet[string(clusterAndNamespace)] = append(nameSet[string(clusterAndNamespace)], string(changefeedID))
	}
	if len(nameSet) == 0 {
		// check etcd KV of CDC <= v6.1
		resp, err = cli.Get(ctx, CDCPrefixV61, clientv3.WithPrefix())
		if err != nil {
			return nil, errors.Trace(err)
		}
		for _, kv := range resp.Kvs {
			// example: /tidb/cdc/changefeed/info/<changefeedID>
			k := kv.Key[len(CDCPrefixV61):]
			if len(k) == 0 {
				continue
			}
			if !isActiveCDCChangefeed(kv.Value) {
				continue
			}

			nameSet["<nil>"] = append(nameSet["<nil>"], string(k))
		}
	}

	return &CDCNameSet{nameSet}, nil
}

type onlyState struct {
	State string `json:"state"`
}

func isActiveCDCChangefeed(jsonBytes []byte) bool {
	s := onlyState{}
	err := json.Unmarshal(jsonBytes, &s)
	if err != nil {
		// maybe a compatible issue, skip this key
		log.L().Error("unmarshal etcd value failed when check CDC changefeed, will skip this key",
			zap.ByteString("value", jsonBytes),
			zap.Error(err))
		return false
	}
	switch s.State {
	// https://docs.pingcap.com/zh/tidb/stable/ticdc-changefeed-overview
	case "normal", "stopped", "error", "warning":
		return true
	default:
		return false
	}
}
