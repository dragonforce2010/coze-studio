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

package appinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/volcengine/volcengine-go-sdk/service/ark"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
	"log"

	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/coze-dev/coze-studio/backend/infra/contract/chatmodel"
	"github.com/coze-dev/coze-studio/backend/infra/contract/modelmgr"
	"github.com/coze-dev/coze-studio/backend/infra/impl/modelmgr/static"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
)

func initModelMgr() (modelmgr.Manager, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	staticModel, err := initModelByTemplate(wd, "resources/conf/model")
	if err != nil {
		return nil, err
	}

	envModel, err := initModelByEnv(wd, "resources/conf/model/template")
	if err != nil {
		return nil, err
	}

	ak, sk, region := "", "", "cn-beijing"
	customModel, err := initModelByAKSK(ak, sk, region)

	all := append(staticModel, envModel...)
	all = append(all, customModel...)
	if err := fillModelContent(all); err != nil {
		return nil, err
	}

	mgr, err := static.NewModelMgr(all)
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

func initModelByAKSK(ak string, sk string, region string) ([]*modelmgr.Model, error) {
	config := volcengine.NewConfig().
		WithRegion(region).
		WithCredentials(credentials.NewStaticCredentials(ak, sk, ""))
	// 创建一个新的 session
	sess, err := session.NewSession(config)
	if err != nil {
		panic(err)
	}

	// 使用 New 函数创建 ARK 服务的 client 实例
	svc := ark.New(sess)

	// 创建请求参数，若无需过滤条件可传 nil
	input := &ark.ListEndpointsInput{
		Filter: &ark.FilterForListEndpointsInput{
			// 可根据需要设置过滤条件，示例如下：
			// Name: volcengine.String("<ENDPOINT_NAME>"),
			// Ids: []*string{volcengine.String("<ENDPOINT_ID_1>"), volcengine.String("<ENDPOINT_ID_2>")},
		},
	}

	// 带上下文调用 ListEndpoints 方法
	ctx := context.Background()
	output, err := svc.ListEndpointsWithContext(ctx, input)
	fmt.Println(output.Items[0])
	resp := make([]*modelmgr.Model, 0, len(output.Items))

	for i := 0; i < len(output.Items); i++ {
		jsonTemplate := `{
  "id": %d,
  "name": "%s",
  "icon_uri": "default_icon/doubao_v2.png",
  "icon_url": "",
  "description": {
    "zh": "%s",
    "en": ""
  },
  "default_parameters": [
    {
      "name": "temperature",
      "label": {
        "zh": "生成随机性",
        "en": "Temperature"
      },
      "desc": {
        "zh": "- **temperature**: 调高温度会使得模型的输出更多样性和创新性，反之，降低温度会使输出内容更加遵循指令要求但减少多样性。建议不要与“Top p”同时调整。",
        "en": "**Temperature**:\n\n- When you increase this value, the model outputs more diverse and innovative content; when you decrease it, the model outputs less diverse content that strictly follows the given instructions.\n- It is recommended not to adjust this value with \"Top p\" at the same time."
      },
      "type": "float",
      "min": "0",
      "max": "1",
      "default_val": {
        "balance": "0.8",
        "creative": "1",
        "default_val": "1.0",
        "precise": "0.3"
      },
      "precision": 1,
      "options": [],
      "style": {
        "widget": "slider",
        "label": {
          "zh": "生成多样性",
          "en": "Generation diversity"
        }
      }
    },
    {
      "name": "max_tokens",
      "label": {
        "zh": "最大回复长度",
        "en": "Response max length"
      },
      "desc": {
        "zh": "控制模型输出的Tokens 长度上限。通常 100 Tokens 约等于 150 个中文汉字。",
        "en": "You can specify the maximum length of the tokens output through this value. Typically, 100 tokens are approximately equal to 150 Chinese characters."
      },
      "type": "int",
      "min": "1",
      "max": "4096",
      "default_val": {
        "default_val": "4096"
      },
      "options": [],
      "style": {
        "widget": "slider",
        "label": {
          "zh": "输入及输出设置",
          "en": "Input and output settings"
        }
      }
    },
    {
      "name": "top_p",
      "label": {
        "zh": "Top P",
        "en": "Top P"
      },
      "desc": {
        "zh": "- **Top p 为累计概率**: 模型在生成输出时会从概率最高的词汇开始选择，直到这些词汇的总概率累积达到Top p 值。这样可以限制模型只选择这些高概率的词汇，从而控制输出内容的多样性。建议不要与“生成随机性”同时调整。",
        "en": "**Top P**:\n\n- An alternative to sampling with temperature, where only tokens within the top p probability mass are considered. For example, 0.1 means only the top 10%% probability mass tokens are considered.\n- We recommend altering this or temperature, but not both."
      },
      "type": "float",
      "min": "0",
      "max": "1",
      "default_val": {
        "default_val": "0.7"
      },
      "precision": 2,
      "options": [],
      "style": {
        "widget": "slider",
        "label": {
          "zh": "生成多样性",
          "en": "Generation diversity"
        }
      }
    },
    {
      "name": "response_format",
      "label": {
        "zh": "输出格式",
        "en": "Response format"
      },
      "desc": {
        "zh": "- **JSON**: 将引导模型使用JSON格式输出",
        "en": "**Response Format**:\n\n- **JSON**: Uses JSON format for replies"
      },
      "type": "int",
      "min": "",
      "max": "",
      "default_val": {
        "default_val": "0"
      },
      "options": [
        {
          "label": "Text",
          "value": "0"
        },
        {
          "label": "JSON",
          "value": "1"
        }
      ],
      "style": {
        "widget": "radio_buttons",
        "label": {
          "zh": "输入及输出设置",
          "en": "Input and output settings"
        }
      }
    }
  ],
  "meta": {
    "protocol": "ark",
    "capability": {
      "function_call": true,
      "input_modal": [
        "text",
        "image",
        "video"
      ],
      "input_tokens": 224000,
      "json_mode": true,
      "max_tokens": 256000,
      "output_modal": [
        "text"
      ],
      "output_tokens": 32000,
      "prefix_caching": true,
      "reasoning": true,
      "prefill_response": false
    },
    "conn_config": {
      "base_url": "https://ark.cn-beijing.volces.com/api/v3/",
      "api_key": "",
      "timeout": 0,
      "model": "%s",
      "temperature": 0.1,
      "frequency_penalty": 0,
      "presence_penalty": 0,
      "max_tokens": 4096,
      "top_p": 0.7,
      "top_k": 0,
      "stop": [],
      "enable_thinking": true,
      "ark": {
        "region": "",
        "access_key": "%s",
        "secret_key": "%s",
        "retry_times": null,
        "custom_header": {}
      },
      "custom": {}
    },
    "status": 0
  }
}`
		itemName := *output.Items[i].Name // 对应 output.Items[0].Name（需解引用指针：*output.Items[0].Name）
		itemId := *output.Items[i].Id     // 对应 output.Items[0].Id（需解引用指针：*output.Items[0].Id）
		itemDescribtion := *output.Items[i].Description

		// 3. 用 fmt.Sprintf 填充变量到模板
		jsonData := fmt.Sprintf(jsonTemplate, i, itemName, itemDescribtion, itemId, ak, sk)

		var content modelmgr.Model
		if err := json.Unmarshal([]byte(jsonData), &content); err != nil {
			log.Fatalf("JSON反序列化失败: %v", err)
			return nil, err
		}
		resp = append(resp, &content)

		//yamlBytes, err := yaml.Marshal(content)
		//if err != nil {
		//	fmt.Printf("YAML 序列化失败: %v\n", err)
		//	return nil, err
		//}
		//
		//var path = fmt.Sprintf("/output%d.yaml", i)
		//if err := os.WriteFile(configPath+path, yamlBytes, 0644); err != nil {
		//	fmt.Printf("写入文件失败: %v\n", err)
		//	return nil, err
		//}

	}
	return resp, nil
}

func initModelByTemplate(wd, configPath string) ([]*modelmgr.Model, error) {
	configRoot := filepath.Join(wd, configPath)
	staticModel, err := readDirYaml[modelmgr.Model](configRoot)
	if err != nil {
		return nil, err
	}
	return staticModel, nil
}

func initModelByEnv(wd, templatePath string) (modelEntities []*modelmgr.Model, err error) {
	entityRoot := filepath.Join(wd, templatePath)

	for i := -1; i < 1000; i++ {
		rawProtocol := os.Getenv(concatEnvKey(modelProtocolPrefix, i))
		if rawProtocol == "" {
			if i < 0 {
				continue
			} else {
				break
			}
		}

		protocol := chatmodel.Protocol(rawProtocol)
		info, valid := getModelEnv(i)
		if !valid {
			break
		}

		mapping, found := modelMapping[protocol]
		if !found {
			return nil, fmt.Errorf("[initModelByEnv] unsupport protocol: %s", rawProtocol)
		}

		switch protocol {
		case chatmodel.ProtocolArk:
			fileSuffix, foundTemplate := mapping[info.modelName]
			if !foundTemplate {
				logs.Warnf("[initModelByEnv] unsupport model=%s, using default config", info.modelName)
			}
			modelEntity, err := readYaml[modelmgr.Model](filepath.Join(entityRoot, concatTemplateFileName("model_template_ark", fileSuffix)))
			if err != nil {
				return nil, err
			}
			id, err := strconv.ParseInt(info.id, 10, 64)
			if err != nil {
				return nil, err
			}

			modelEntity.ID = id
			if !foundTemplate {
				modelEntity.Name = info.modelName
			}
			modelEntity.Meta.ConnConfig.Model = info.modelID
			modelEntity.Meta.ConnConfig.APIKey = info.apiKey
			modelEntity.Meta.ConnConfig.BaseURL = info.baseURL

			modelEntities = append(modelEntities, modelEntity)

		default:
			return nil, fmt.Errorf("[initModelByEnv] unsupport protocol: %s", rawProtocol)
		}
	}

	return modelEntities, nil
}

type envModelInfo struct {
	id, modelName, modelID, apiKey, baseURL string
}

func getModelEnv(idx int) (info envModelInfo, valid bool) {
	info.id = os.Getenv(concatEnvKey(modelOpenCozeIDPrefix, idx))
	info.modelName = os.Getenv(concatEnvKey(modelNamePrefix, idx))
	info.modelID = os.Getenv(concatEnvKey(modelIDPrefix, idx))
	info.apiKey = os.Getenv(concatEnvKey(modelApiKeyPrefix, idx))
	info.baseURL = os.Getenv(concatEnvKey(modelBaseURLPrefix, idx))
	valid = info.modelName != "" && info.modelID != "" && info.apiKey != ""
	return
}

func readDirYaml[T any](dir string) ([]*T, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	resp := make([]*T, 0, len(des))
	for _, file := range des {
		if file.IsDir() {
			continue
		}
		if strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml") {
			filePath := filepath.Join(dir, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			var content T
			if err := yaml.Unmarshal(data, &content); err != nil {
				return nil, err
			}
			resp = append(resp, &content)
		}
	}
	return resp, nil
}

func readYaml[T any](fPath string) (*T, error) {
	data, err := os.ReadFile(fPath)
	if err != nil {
		return nil, err
	}
	var content T
	if err := yaml.Unmarshal(data, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

func concatEnvKey(prefix string, idx int) string {
	if idx < 0 {
		return prefix
	}
	return fmt.Sprintf("%s_%d", prefix, idx)
}

func concatTemplateFileName(prefix, suffix string) string {
	if suffix == "" {
		return prefix + ".yaml"
	}
	return prefix + "_" + suffix + ".yaml"
}

const (
	modelProtocolPrefix   = "MODEL_PROTOCOL"    // model protocol
	modelOpenCozeIDPrefix = "MODEL_OPENCOZE_ID" // opencoze model id
	modelNamePrefix       = "MODEL_NAME"        // model name,
	modelIDPrefix         = "MODEL_ID"          // model in conn config
	modelApiKeyPrefix     = "MODEL_API_KEY"     // model api key
	modelBaseURLPrefix    = "MODEL_BASE_URL"    // model base url
)

var modelMapping = map[chatmodel.Protocol]map[string]string{
	chatmodel.ProtocolArk: {
		"doubao-seed-1.6":                "doubao-seed-1.6",
		"doubao-seed-1.6-flash":          "doubao-seed-1.6-flash",
		"doubao-seed-1.6-thinking":       "doubao-seed-1.6-thinking",
		"doubao-1.5-thinking-vision-pro": "doubao-1.5-thinking-vision-pro",
		"doubao-1.5-thinking-pro":        "doubao-1.5-thinking-pro",
		"doubao-1.5-vision-pro":          "doubao-1.5-vision-pro",
		"doubao-1.5-vision-lite":         "doubao-1.5-vision-lite",
		"doubao-1.5-pro-32k":             "doubao-1.5-pro-32k",
		"doubao-1.5-pro-256k":            "doubao-1.5-pro-256k",
		"doubao-1.5-lite":                "doubao-1.5-lite",
		"deepseek-r1":                    "volc_deepseek-r1",
		"deepseek-v3":                    "volc_deepseek-v3",
	},
}

func fillModelContent(items []*modelmgr.Model) error {
	for i := range items {
		item := items[i]
		if item.Meta.Status == modelmgr.StatusDefault {
			item.Meta.Status = modelmgr.StatusInUse
		}

		if item.IconURI == "" && item.IconURL == "" {
			return fmt.Errorf("missing icon URI or icon URL, id=%d", item.ID)
		}
	}

	return nil
}
