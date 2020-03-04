// Copyright 2019 Liquidata, Inc.
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

package tblcmds

import (
	"context"

	eventsapi "github.com/liquidata-inc/dolt/go/gen/proto/dolt/services/eventsapi/v1alpha1"
	"github.com/liquidata-inc/dolt/go/libraries/utils/filesys"

	"github.com/liquidata-inc/dolt/go/cmd/dolt/cli"
	"github.com/liquidata-inc/dolt/go/cmd/dolt/commands"
	"github.com/liquidata-inc/dolt/go/cmd/dolt/errhand"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/doltdb"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/env"
	"github.com/liquidata-inc/dolt/go/libraries/utils/argparser"
)

var tblCpShortDesc = "Makes a copy of a table"
var tblCpLongDesc = `The dolt table cp command makes a copy of a table at a given commit. If a commit is not specified the copy is made of the table from the current working set.

If a table exists at the target location this command will fail unless the {{.EmphasisLeft}}--force|-f{{.EmphasisRight}} flag is provided.  In this case the table at the target location will be overwritten with the copied table.

All changes will be applied to the working tables and will need to be staged using {{.EmphasisLeft}}dolt add{{.EmphasisRight}} and committed using {{.EmphasisLeft}}dolt commit{{.EmphasisRight}}.
`

var tblCpSynopsis = []string{
	"[-f] [{{.LessThan}}commit{{.GreaterThan}}] {{.LessThan}}oldtable{{.GreaterThan}} {{.LessThan}}newtable{{.GreaterThan}}",
}

var tblCpDocumentation = cli.CommandDocumentation{
	ShortDesc: tblCpShortDesc,
	LongDesc: tblCpLongDesc,
	Synopsis: tblCpSynopsis,
}

type CpCmd struct{}

// Name is returns the name of the Dolt cli command. This is what is used on the command line to invoke the command
func (cmd CpCmd) Name() string {
	return "cp"
}

// Description returns a description of the command
func (cmd CpCmd) Description() string {
	return "Copies a table"
}

// CreateMarkdown creates a markdown file containing the helptext for the command at the given path
func (cmd CpCmd) CreateMarkdown(fs filesys.Filesys, path, commandStr string) error {
	ap := cmd.createArgParser()
	return commands.CreateMarkdown(fs, path, commandStr, tblCpDocumentation, ap)
}

func (cmd CpCmd) createArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"commit", "The state at which point the table whill be copied."})
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"oldtable", "The table being copied."})
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"newtable", "The destination where the table is being copied to."})
	ap.SupportsFlag(forceParam, "f", "If data already exists in the destination, the Force flag will allow the target to be overwritten.")
	return ap
}

// EventType returns the type of the event to log
func (cmd CpCmd) EventType() eventsapi.ClientEventType {
	return eventsapi.ClientEventType_TABLE_CP
}

// Exec executes the command
func (cmd CpCmd) Exec(ctx context.Context, commandStr string, args []string, dEnv *env.DoltEnv) int {
	ap := cmd.createArgParser()
	help, usage := cli.HelpAndUsagePrinters(commandStr, tblCpDocumentation, ap)
	apr := cli.ParseArgs(ap, args, help)

	if apr.NArg() < 2 || apr.NArg() > 3 {
		usage()
		return 1
	}

	force := apr.Contains(forceParam)
	working, verr := commands.GetWorkingWithVErr(dEnv)
	if verr != nil {
		return commands.HandleVErrAndExitCode(verr, usage)
	}

	root := working

	var old, new string
	if apr.NArg() == 3 {
		var cm *doltdb.Commit
		cm, verr = commands.ResolveCommitWithVErr(dEnv, apr.Arg(0), dEnv.RepoState.CWBHeadRef().String())
		if verr != nil {
			return commands.HandleVErrAndExitCode(verr, usage)
		}
		var err error
		root, err = cm.GetRootValue()

		if err != nil {
			verr = errhand.BuildDError("error: failed to get root value").AddCause(err).Build()
			return commands.HandleVErrAndExitCode(verr, usage)
		}

		old, new = apr.Arg(1), apr.Arg(2)
	} else {
		old, new = apr.Arg(0), apr.Arg(1)
	}

	if err := ValidateTableNameForCreate(new); err != nil {
		return commands.HandleVErrAndExitCode(err, usage)
	}

	tbl, ok, err := root.GetTable(ctx, old)

	if err != nil {
		verr = errhand.BuildDError("error: failed to get table").AddCause(err).Build()
		return commands.HandleVErrAndExitCode(verr, usage)
	}

	if !ok {
		verr = errhand.BuildDError("Table '%s' not found in root", old).Build()
		return commands.HandleVErrAndExitCode(verr, usage)
	}

	has, err := working.HasTable(ctx, new)

	if err != nil {
		verr = errhand.BuildDError("error: failed to get tables").AddCause(err).Build()
		return commands.HandleVErrAndExitCode(verr, usage)
	} else if !force && has {
		verr = errhand.BuildDError("Data already exists in '%s'.  Use -f to overwrite.", new).Build()
		return commands.HandleVErrAndExitCode(verr, usage)
	}

	working, err = working.PutTable(ctx, new, tbl)
	if err != nil {
		verr = errhand.BuildDError("error; failed to write tables back to database").Build()
		return commands.HandleVErrAndExitCode(verr, usage)
	}

	return commands.HandleVErrAndExitCode(commands.UpdateWorkingWithVErr(dEnv, working), usage)
}
