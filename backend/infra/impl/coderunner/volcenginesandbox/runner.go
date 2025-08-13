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

package volcenginesandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-studio/backend/infra/contract/coderunner"
	"github.com/coze-dev/coze-studio/backend/pkg/logs"
	"github.com/volcengine/volcengine-go-sdk/service/vefaas"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

var _ coderunner.Runner = (*runner)(nil)

var pythonCode = `
import asyncio
import json
import sys

class Args:
    def __init__(self, params):
        self.params = params

class Output(dict):
    pass

%s

try:
    result = asyncio.run(main( Args(json.loads('%s'))))
    print(json.dumps(result))
except Exception as  e:
    print(f"{type(e).__name__}: {str(e)}", file=sys.stderr)
    sys.exit(1)

`

func NewRunner(c *Config) coderunner.Runner {
	cfg := volcengine.NewConfig().
		WithCredentials(credentials.NewStaticCredentials(c.AccessKey, c.SecretKey, "")).
		WithRegion(c.Region)
	sess, err := session.NewSession(cfg)
	if err != nil {
		logs.CtxErrorf(context.Background(), "new session failed: %v", err)
		return nil
	}
	faasClient := vefaas.New(sess)
	return &runner{
		cli:               faasClient,
		sandboxFunctionId: c.FuncId,
	}
}

type Config struct {
	AccessKey string
	SecretKey string
	Region    string
	FuncId    string
}

type runner struct {
	cli               *vefaas.VEFAAS
	sandboxFunctionId string
}

type RunResult struct {
	Status        string `json:"status"`
	Message       string `json:"message"`
	CompileResult struct {
		Status        string  `json:"status"`
		ExecutionTime float64 `json:"execution_time"`
		CPUTime       float64 `json:"cpu_time"`
		ReturnCode    int     `json:"return_code"`
		Stdout        string  `json:"stdout"`
		Stderr        string  `json:"stderr"`
	} `json:"compile_result"`
	RunResult struct {
		Status        string  `json:"status"`
		ExecutionTime float64 `json:"execution_time"`
		CPUTime       float64 `json:"cpu_time"`
		ReturnCode    int     `json:"return_code"`
		Stdout        string  `json:"stdout"`
		Stderr        string  `json:"stderr"`
	} `json:"run_result"`
	ExecutorPodName string `json:"executor_pod_name"`
	Files           struct {
	} `json:"files"`
}

func (r *runner) Run(ctx context.Context, request *coderunner.RunRequest) (*coderunner.RunResponse, error) {
	sandboxId, err := r.getSandbox(ctx)
	if err != nil {
		return nil, err
	}
	jsonData, err := sonic.MarshalString(request.Params)
	if err != nil {
		return nil, err
	}

	code := fmt.Sprintf(pythonCode, request.Code, jsonData)

	runData := map[string]any{
		"code":     code,
		"language": strings.ToLower(string(request.Language)),
	}
	runDataStr, err := sonic.MarshalString(runData)
	if err != nil {
		return nil, err
	}

	logs.CtxInfof(ctx, "run code: %s", runDataStr)
	out, err := r.cli.RunCodeWithContext(ctx, &vefaas.RunCodeInput{
		SandboxId:  sandboxId,
		FunctionId: &r.sandboxFunctionId,
		Data:       &runDataStr,
	})

	if err != nil {
		return nil, err
	}

	if out.Result == nil {
		return nil, errors.New("no result")
	}
	ret := &RunResult{}
	err = sonic.UnmarshalString(*out.Result, ret)
	if err != nil {
		return nil, err
	}
	if ret.RunResult.Stderr != "" {
		return nil, fmt.Errorf("failed to run python script err: %s", ret.RunResult.Stderr)
	}
	result := make(map[string]any)
	err = sonic.UnmarshalString(ret.RunResult.Stdout, &result)
	if err != nil {
		return nil, err
	}
	return &coderunner.RunResponse{
		Result: result,
	}, nil
}

func (r *runner) getSandbox(ctx context.Context) (*string, error) {
	out, err := r.cli.ListSandboxesWithContext(ctx, &vefaas.ListSandboxesInput{
		FunctionId: &r.sandboxFunctionId,
	})
	if err != nil {
		return nil, err
	}
	if len(out.Sandboxes) == 0 {
		_, err := r.cli.CreateSandboxWithContext(ctx, &vefaas.CreateSandboxInput{
			FunctionId: &r.sandboxFunctionId,
			Metadata:   &vefaas.MetadataForCreateSandboxInput{},
		})
		if err != nil {
			return nil, err
		}
	}

	for _, sandbox := range out.Sandboxes {
		if sandbox.Status == nil || *sandbox.Status != "Ready" {
			continue
		}
		return sandbox.Id, nil
	}

	// if no ready sandbox, wait for it
	sandboxChan := make(chan *string)
	go func() {
		tk := time.NewTicker(3 * time.Second)
		defer tk.Stop()

		for {
			select {
			case <-tk.C:
				out, err := r.cli.ListSandboxesWithContext(ctx, &vefaas.ListSandboxesInput{
					FunctionId: &r.sandboxFunctionId,
				})
				if err != nil {
					continue
				}
				for _, sandbox := range out.Sandboxes {
					if sandbox.Status == nil || *sandbox.Status != "Ready" {
						continue
					}
					sandboxChan <- sandbox.Id
				}
			case <-time.After(30 * time.Second):
				sandboxChan <- nil
			default:
				time.Sleep(1 * time.Second)
			}
		}
	}()

	id := <-sandboxChan
	if id == nil {
		return nil, errors.New("no ready sandbox found")
	}
	return id, nil
}
