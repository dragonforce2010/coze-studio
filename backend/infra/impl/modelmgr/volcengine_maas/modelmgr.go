/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package volcengine_maas

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/coze-dev/coze-studio/backend/infra/contract/chatmodel"
	"github.com/coze-dev/coze-studio/backend/infra/contract/modelmgr"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-studio/backend/pkg/lang/sets"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/coze-dev/coze-studio/backend/types/consts"
	"github.com/volcengine/volcengine-go-sdk/service/ark"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

var _ modelmgr.Manager = (*volcModelManager)(nil)

var preEndpointID = map[string]string{
	"Doubao-1.5-Vision-Pro":          "doubao-1-5-vision-pro-250328",
	"Doubao-Seed-1.6-Thinking":       "doubao-seed-1-6-thinking-250715",
	"Doubao-Seed-1.6-Flash":          "doubao-seed-1-6-flash-250715",
	"Doubao-1.5-Pro-32k":             "doubao-1-5-pro-32k-250115",
	"Doubao-1.5-Thinking-Vision-Pro": "doubao-1-5-thinking-vision-pro-250428",
	"Doubao-1.5-Thinking-Pro":        "doubao-1-5-thinking-pro-250415",
	"Doubao-Seed-1.6":                "doubao-seed-1-6-250615",
	"Doubao-1.5-Pro-256k":            "doubao-1-5-pro-256k-250115",
	"Doubao-1.5-Lite":                "doubao-1-5-lite-32k-250115",
	"Doubao-1.5-Vision-Lite":         "doubao-1-5-vision-lite-250315",
	"Deepseek-R1-VolcEngine":         "deepseek-r1-250528",
	"Deepseek-V3-VolcEngine":         "deepseek-v3-250324",
}

type volcModelManager struct {
	models       []*modelmgr.Model
	modelMapping map[int64]*modelmgr.Model
	arkClient    *ark.ARK
}

func NewModelMgr(staticModels []*modelmgr.Model) (modelmgr.Manager, error) {

	cfg := volcengine.NewConfig().
		WithCredentials(credentials.NewStaticCredentials(os.Getenv(consts.VolcengineAccessKey), os.Getenv(consts.VolcengineSecretKey), "")).
		WithRegion(os.Getenv(consts.VolcengineRegion))

	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, err
	}
	svc := ark.New(sess)

	mapping := make(map[int64]*modelmgr.Model, len(staticModels))
	for i := range staticModels {
		mapping[staticModels[i].ID] = staticModels[i]
	}

	manager := &volcModelManager{
		arkClient:    svc,
		models:       staticModels,
		modelMapping: mapping,
	}
	manager.initModelList(context.Background())
	return manager, nil
}

func (v *volcModelManager) initModelList(ctx context.Context) error {
	newModels := make([]*modelmgr.Model, 0)
	logs.CtxInfof(ctx, "init model list, model count: %d", len(v.models))
	for i := range v.models {
		logs.CtxInfof(ctx, "init model list, model name: %s, pre endpoint id: %s", v.models[i].Name, preEndpointID[v.models[i].Name])
		m := v.models[i]
		if m.Meta.Protocol != chatmodel.ProtocolArk || !strings.Contains(m.Meta.ConnConfig.BaseURL, "volces") ||
			preEndpointID[m.Name] == "" {
			continue
		}
		// item, err := v.listEndpoints(ctx, m.Name)
		// if err != nil {
		// 	continue
		// }
		// m.Meta.ConnConfig.Model = *item.Id
		m.Meta.ConnConfig.Model = preEndpointID[m.Name]
		m.Meta.ConnConfig.APIKey = os.Getenv(consts.VolcengineMAASAPIKey)
		m.Meta.Status = modelmgr.StatusInUse
		newModels = append(newModels, m)
	}
	logs.CtxInfof(ctx, "init model list, new model count: %d", len(newModels))
	v.models = newModels
	mapping := make(map[int64]*modelmgr.Model, len(newModels))
	for i := range newModels {
		mapping[newModels[i].ID] = newModels[i]
	}
	v.modelMapping = mapping
	return nil
}

func (v *volcModelManager) listEndpoints(ctx context.Context, modelName string) (*ark.ItemForListEndpointsOutput, error) {
	m := strings.ReplaceAll(strings.ReplaceAll(modelName, ".", "-"), "-VolcEngine", "")
	input := &ark.ListEndpointsInput{
		Filter: &ark.FilterForListEndpointsInput{
			FoundationModelName: volcengine.String(m),
		},
	}
	resp, err := v.arkClient.ListEndpointsWithContext(ctx, input)
	if err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("model %s not found", modelName)
	}
	return resp.Items[0], nil
}

func (v *volcModelManager) ListModel(ctx context.Context, req *modelmgr.ListModelRequest) (*modelmgr.ListModelResponse, error) {
	startIdx := 0
	if req.Cursor != nil {
		start, err := strconv.ParseInt(*req.Cursor, 10, 64)
		if err != nil {
			return nil, err
		}
		startIdx = int(start)
	}

	limit := req.Limit
	if limit == 0 {
		limit = 100
	}

	var (
		i        int
		respList []*modelmgr.Model
		statSet  = sets.FromSlice(req.Status)
	)

	for i = startIdx; i < len(v.models) && len(respList) < limit; i++ {
		m := v.models[i]
		if req.FuzzyModelName != nil && !strings.Contains(m.Name, *req.FuzzyModelName) {
			continue
		}
		if len(statSet) > 0 && !statSet.Contains(m.Meta.Status) {
			continue
		}
		respList = append(respList, m)
	}

	resp := &modelmgr.ListModelResponse{
		ModelList: respList,
	}
	resp.HasMore = i != len(v.models)
	if resp.HasMore {
		resp.NextCursor = ptr.Of(strconv.FormatInt(int64(i), 10))
	}

	return resp, nil
}

func (v *volcModelManager) ListInUseModel(ctx context.Context, limit int, Cursor *string) (*modelmgr.ListModelResponse, error) {
	return v.ListModel(ctx, &modelmgr.ListModelRequest{
		Status: []modelmgr.ModelStatus{modelmgr.StatusInUse},
		Limit:  limit,
		Cursor: Cursor,
	})
}

func (v *volcModelManager) MGetModelByID(ctx context.Context, req *modelmgr.MGetModelRequest) ([]*modelmgr.Model, error) {
	resp := make([]*modelmgr.Model, 0)
	for _, id := range req.IDs {
		if m, found := v.modelMapping[id]; found {
			resp = append(resp, m)
		}
	}
	return resp, nil
}
