// Copyright © 2018 Matt Tesauro <matt.tesauro@owasp.org>
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

package cmd

import (
	"fmt"

	g "github.com/mtesauro/gasp"
	d "github.com/mtesauro/gasp-docker/gdocker"
	"github.com/spf13/cobra"
)

// Vars to handle command-line args
var Profile, AppName, Src, Rpt, Vol, AppProfile,
	ToolProfile, Target, PipeType, Loc, Params string
var Keep, DryRun bool

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the provided named pipeline aka profile from master.yaml",
	Long: `Run the provide named pipeline aka profile from master.yaml

For example:
  gasp-docker run -profile="pre-launch"

would run the pipeline called "pre-launch" as defined in master.yaml 

`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("\nrun called")

		// Take the run flags and fill the EventArgs struct
		tc := make(map[string]string)
		ev := g.EventArgs{
			Profile,
			AppName,
			Target,
			DryRun,
			Keep,
			Vol,
			Src,
			Rpt,
			AppProfile,
			ToolProfile,
			Loc,
			Params,
			tc,
		}

		// Load the pipeline for a run
		d.LoadPipeline(&ev)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	runCmd.Flags().StringVarP(&Profile,
		"profile",
		"p",
		"",
		"<required> The named pipeline aka profile from master.yaml to run")
	runCmd.MarkFlagRequired("profile")

	runCmd.Flags().StringVarP(&AppName,
		"app-name",
		"a",
		"",
		"<required> The name of the app the application that is the target of this pipeline run")
	runCmd.MarkFlagRequired("app-name")

	runCmd.Flags().StringVarP(&Target,
		"target",
		"t",
		"TBD",
		"The target to use for this pipeline run, generally a repo URL for SAST or URL for DAST")
	//runCmd.MarkFlagRequired("target")
	// TODO: Fix ^ so targets are defined and can be provided with a simple name at runtime

	//runCmd.Flags().StringVarP(&PipeType,
	//	"pipe-type",
	//	"y",
	//	"SAST",
	//	"<required> Type of pipeline run - currently SAST (static) or DAST (dynamic) are supported")
	//runCmd.MarkFlagRequired("pipe-type")

	runCmd.Flags().BoolVarP(&DryRun,
		"dry-run",
		"d",
		false,
		"If present, run he pipeline without actually launching containers, basically loging only")

	runCmd.Flags().BoolVarP(&Keep,
		"keep",
		"k",
		false,
		"If present, keep any containers used during the pipeline run")

	runCmd.Flags().StringVarP(&Vol,
		"volume",
		"v",
		"none",
		"The full path to a local directory to use for all pipeline run files instead of an ephemeral data container")

	runCmd.Flags().StringVarP(&Src,
		"source",
		"s",
		"none",
		"The full path to a local directory which contains source code for SAST pipeline runs")

	runCmd.Flags().StringVarP(&Rpt,
		"reports",
		"r",
		"none",
		"The full path to a local directory which contains source code for SAST pipeline runs")

	runCmd.Flags().StringVarP(&AppProfile,
		"app-profile",
		"f",
		"none",
		"The application specific named pipeline (profile) to use for this run in [app-name]-pipeline.yaml")

	runCmd.Flags().StringVarP(&ToolProfile,
		"tool-profile",
		"o",
		"none",
		"The custom tool profile to override the profiles defined in secpipeline-config.yaml for this run")

	runCmd.Flags().StringVarP(&Loc,
		"location",
		"l",
		"/opt/appsecpipeline/source",
		"Path to where the sourcecode is in the container")

	runCmd.Flags().StringVarP(&Params,
		"params",
		"m",
		"",
		"Required parametetrs for the pipeline tools in this run")

}
