// Copyright 2023 Dolthub, Inc.
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

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/dolthub/dolt/go/cmd/dolt/cli"
	"github.com/dolthub/dolt/go/cmd/dolt/errhand"
	eventsapi "github.com/dolthub/dolt/go/gen/proto/dolt/services/eventsapi/v1alpha1"
	"github.com/dolthub/dolt/go/libraries/doltcore/dbfactory"
	"github.com/dolthub/dolt/go/libraries/doltcore/env"
	"github.com/dolthub/dolt/go/libraries/utils/argparser"
	"github.com/dolthub/dolt/go/libraries/utils/config"
)

var profileDocs = cli.CommandDocumentationContent{
	ShortDesc: "Manage dolt profiles for CLI global options.",
	LongDesc: `With no arguments, shows a list of existing profiles. Several subcommands are available to perform operations on the profiles.
{{.EmphasisLeft}}add{{.EmphasisRight}}
Adds a profile named {{.LessThan}}name{{.GreaterThan}}. If the profile already exists, it will be overwritten.

{{.EmphasisLeft}}remove{{.EmphasisRight}}, {{.EmphasisLeft}}rm{{.EmphasisRight}}
Remove the profile named {{.LessThan}}name{{.GreaterThan}}.`,
	Synopsis: []string{
		"",
		"add [-u {{.LessThan}}user{{.GreaterThan}}] [-p {{.LessThan}}password{{.GreaterThan}}] [--host {{.LessThan}}host{{.GreaterThan}}] [--port {{.LessThan}}port{{.GreaterThan}}] [--no-tls] [--data-dir {{.LessThan}}directory{{.GreaterThan}}] [--doltcfg-dir {{.LessThan}}directory{{.GreaterThan}}] [--privilege-file {{.LessThan}}privilege file{{.GreaterThan}}] [--branch-control-file {{.LessThan}}branch control file{{.GreaterThan}}] [--use-db {{.LessThan}}database{{.GreaterThan}}] {{.LessThan}}name{{.GreaterThan}}",
		"remove {{.LessThan}}name{{.GreaterThan}}",
	},
}

const (
	addProfileId         = "add"
	removeProfileId      = "remove"
	removeProfileShortId = "rm"
	GlobalCfgProfileKey  = "profile"
)

type ProfileCmd struct{}

// Name returns the name of the Dolt cli command. This is what is used on the command line to invoke the command
func (cmd ProfileCmd) Name() string {
	return "profile"
}

// Description returns a description of the command
func (cmd ProfileCmd) Description() string {
	return "Manage dolt profiles for CLI global options."
}

func (cmd ProfileCmd) Docs() *cli.CommandDocumentation {
	ap := cmd.ArgParser()
	return cli.NewCommandDocumentation(profileDocs, ap)
}

func (cmd ProfileCmd) ArgParser() *argparser.ArgParser {
	ap := cli.CreateProfileArgParser()
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"name", "Defines the name of the profile to add or remove."})
	return ap
}

// EventType returns the type of the event to log
func (cmd ProfileCmd) EventType() eventsapi.ClientEventType {
	return eventsapi.ClientEventType_PROFILE
}

func (cmd ProfileCmd) RequiresRepo() bool {
	return false
}

// Exec executes the command
func (cmd ProfileCmd) Exec(ctx context.Context, commandStr string, args []string, dEnv *env.DoltEnv, cliCtx cli.CliContext) int {
	ap := cmd.ArgParser()
	help, usage := cli.HelpAndUsagePrinters(cli.CommandDocsForCommandString(commandStr, profileDocs, ap))
	apr := cli.ParseArgsOrDie(ap, args, help)

	var verr errhand.VerboseError

	switch {
	case apr.NArg() == 0:
		verr = printProfiles(dEnv)
	case apr.Arg(0) == addProfileId:
		verr = addProfile(dEnv, apr)
	case apr.Arg(0) == removeProfileId:
		verr = removeProfile(dEnv, apr)
	case apr.Arg(0) == removeProfileShortId:
		verr = removeProfile(dEnv, apr)
	default:
		verr = errhand.BuildDError("").SetPrintUsage().Build()
	}

	return HandleVErrAndExitCode(verr, usage)
}

func addProfile(dEnv *env.DoltEnv, apr *argparser.ArgParseResults) errhand.VerboseError {
	if apr.NArg() != 2 {
		return errhand.BuildDError("Only one profile name can be specified").SetPrintUsage().Build()
	}

	profileName := strings.TrimSpace(apr.Arg(1))

	p := newProfile(apr)
	profStr := p.String()

	cfg, ok := dEnv.Config.GetConfig(env.GlobalConfig)
	if !ok {
		return errhand.BuildDError("error: failed to get global config").Build()
	}
	//TODO: enable config to retrieve json objects instead of just strings
	profilesJSON, err := cfg.GetString(GlobalCfgProfileKey)
	if err != nil {
		if err != config.ErrConfigParamNotFound {
			return errhand.BuildDError("error: failed to get profiles, %s", err).Build()
		} else {
			err = cfg.SetStrings(map[string]string{GlobalCfgProfileKey: "{\"" + profileName + "\"" + ": " + profStr + "}"})
			if err != nil {
				return errhand.BuildDError("error: failed to set profiles, %s", err).Build()
			}
			err = setGlobalConfigPermissions(dEnv)
			if err != nil {
				return errhand.BuildDError("error: failed to set permissions, %s", err).Build()
			}
			return nil
		}
	}

	profilesJSON, err = sjson.Set(profilesJSON, profileName, profStr)
	if err != nil {
		return errhand.BuildDError("error: failed to add profile, %s", err).Build()
	}
	err = cfg.SetStrings(map[string]string{GlobalCfgProfileKey: profilesJSON})
	if err != nil {
		return errhand.BuildDError("error: failed to set profiles, %s", err).Build()
	}
	err = setGlobalConfigPermissions(dEnv)
	if err != nil {
		return errhand.BuildDError("error: failed to set permissions, %s", err).Build()
	}

	return nil
}

func removeProfile(dEnv *env.DoltEnv, apr *argparser.ArgParseResults) errhand.VerboseError {
	if apr.NArg() != 2 {
		return errhand.BuildDError("Only one profile name can be specified").SetPrintUsage().Build()
	}

	profileName := strings.TrimSpace(apr.Arg(1))

	cfg, ok := dEnv.Config.GetConfig(env.GlobalConfig)
	if !ok {
		return errhand.BuildDError("error: failed to get global config").Build()
	}
	profilesJSON, err := cfg.GetString(GlobalCfgProfileKey)
	if err != nil {
		if err == config.ErrConfigParamNotFound {
			return errhand.BuildDError("error: no existing profiles").Build()
		}
		return errhand.BuildDError("error: failed to get profiles, %s", err).Build()
	}

	p := gjson.Get(profilesJSON, profileName)
	if !p.Exists() {
		return errhand.BuildDError("error: profile %s does not exist", profileName).Build()
	}

	profilesJSON, err = sjson.Delete(profilesJSON, profileName)
	if err != nil {
		return errhand.BuildDError("error: failed to remove profile, %s", err).Build()
	}
	err = cfg.SetStrings(map[string]string{GlobalCfgProfileKey: profilesJSON})
	if err != nil {
		return errhand.BuildDError("error: failed to set profiles, %s", err).Build()
	}
	err = setGlobalConfigPermissions(dEnv)
	if err != nil {
		return errhand.BuildDError("error: failed to set permissions, %s", err).Build()
	}

	return nil
}

func printProfiles(dEnv *env.DoltEnv) errhand.VerboseError {
	cfg, ok := dEnv.Config.GetConfig(env.GlobalConfig)
	if !ok {
		return errhand.BuildDError("error: failed to get global config").Build()
	}
	profilesJSON, err := cfg.GetString(GlobalCfgProfileKey)
	if err != nil {
		if err == config.ErrConfigParamNotFound {
			return nil
		}
		return errhand.BuildDError("error: failed to get profiles, %s", err).Build()
	}

	profileMap := gjson.Parse(profilesJSON)
	if !profileMap.Exists() {
		return nil
	}

	for profileName, profile := range profileMap.Map() {
		var p Profile
		var val []byte = []byte(profile.String())
		err := json.Unmarshal([]byte(val), &p)
		if err != nil {
			return errhand.BuildDError("error: failed to unmarshal profile, %s", err).Build()
		}
		prettyPrintProfile(profileName, p)
	}

	return nil
}

func prettyPrintProfile(profileName string, profile Profile) {
	cli.Println(fmt.Sprintf("%s:\n\tuser: %s\n\tpassword: %s\n\thost: %s\n\tport: %s\n\tno-tls: %t\n\tdata-dir: %s\n\tdoltcfg-dir: %s\n\tprivilege-file: %s\n\tbranch-control-file: %s\n\tuse-db: %s\n",
		profileName, profile.User, profile.Password, profile.Host, profile.Port, profile.NoTLS, profile.DataDir, profile.DoltCfgDir, profile.PrivilegeFile, profile.BranchControl, profile.UseDB))
}

// setGlobalConfigPermissions sets permissions on global config file to 0600 to protect potentially sensitive information (credentials)
func setGlobalConfigPermissions(dEnv *env.DoltEnv) error {
	homeDir, err := env.GetCurrentUserHomeDir()
	if err != nil {
		return errhand.BuildDError("error: failed to get home directory: %s", err).Build()
	}
	path, err := dEnv.FS.Abs(filepath.Join(homeDir, dbfactory.DoltDir, env.GlobalConfigFile))
	if err != nil {
		return errhand.BuildDError("error: failed to get global config path: %s", err).Build()
	}
	err = os.Chmod(path, 0600)
	if err != nil {
		return errhand.BuildDError("error: failed to set permissions on global config: %s", err).Build()
	}

	return nil
}

type Profile struct {
	User          string `json:"user"`
	Password      string `json:"password"`
	Host          string `json:"host"`
	Port          string `json:"port"`
	NoTLS         bool   `json:"no-tls"`
	DataDir       string `json:"data-dir"`
	DoltCfgDir    string `json:"doltcfg-dir"`
	PrivilegeFile string `json:"privilege-file"`
	BranchControl string `json:"branch-control-file"`
	UseDB         string `json:"use-db"`
}

func (p Profile) String() string {
	b, err := json.Marshal(p)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func newProfile(apr *argparser.ArgParseResults) Profile {
	return Profile{
		User:          apr.GetValueOrDefault(cli.UserFlag, DefaultUser),
		Password:      apr.GetValueOrDefault(cli.PasswordFlag, ""),
		Host:          apr.GetValueOrDefault(cli.HostFlag, ""),
		Port:          apr.GetValueOrDefault(cli.PortFlag, ""),
		NoTLS:         apr.Contains(cli.NoTLSFlag),
		DataDir:       apr.GetValueOrDefault(DataDirFlag, ""),
		DoltCfgDir:    apr.GetValueOrDefault(CfgDirFlag, ""),
		PrivilegeFile: apr.GetValueOrDefault(PrivsFilePathFlag, ""),
		BranchControl: apr.GetValueOrDefault(BranchCtrlPathFlag, ""),
		UseDB:         apr.GetValueOrDefault(UseDbFlag, ""),
	}
}
