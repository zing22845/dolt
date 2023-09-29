// Copyright 2019-2022 Dolthub, Inc.
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

package dolt_builder

import (
	"context"
	"os"
	"os/exec"
)

var Debug bool

const envDebug = "DEBUG"

func init() {
	if os.Getenv(envDebug) != "" {
		Debug = true
	}
}

func ExecCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	e := exec.CommandContext(ctx, name, arg...)
	if Debug {
		e.Stdout = os.Stdout
		e.Stderr = os.Stderr
	}
	return e
}
