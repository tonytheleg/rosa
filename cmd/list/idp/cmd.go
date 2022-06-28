/*
Copyright (c) 2020 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package idp

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/spf13/cobra"

	"github.com/openshift/rosa/pkg/ocm"
	"github.com/openshift/rosa/pkg/output"
	"github.com/openshift/rosa/pkg/rosa"
)

var Cmd = &cobra.Command{
	Use:     "idps",
	Aliases: []string{"idp"},
	Short:   "List cluster IDPs",
	Long:    "List identity providers for a cluster.",
	Example: `  # List all identity providers on a cluster named "mycluster"
  rosa list idps --cluster=mycluster`,
	Run: run,
}

func init() {
	ocm.AddClusterFlag(Cmd)
	output.AddFlag(Cmd)
}

func run(_ *cobra.Command, _ []string) {
	r := rosa.NewRuntime().WithAWS().WithOCM()
	defer r.Cleanup()

	clusterKey := r.GetClusterKey()

	cluster := r.FetchCluster()
	if cluster.State() != cmv1.ClusterStateReady {
		r.Reporter.Errorf("Cluster '%s' is not yet ready", clusterKey)
		os.Exit(1)
	}

	// Load any existing IDPs for this cluster
	r.Reporter.Debugf("Loading identity providers for cluster '%s'", clusterKey)
	idps, err := r.OCMClient.GetIdentityProviders(cluster.ID())
	if err != nil {
		r.Reporter.Errorf("Failed to get identity providers for cluster '%s': %v", clusterKey, err)
		os.Exit(1)
	}

	if output.HasFlag() {
		err = output.Print(idps)
		if err != nil {
			r.Reporter.Errorf("%s", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if len(idps) == 0 {
		r.Reporter.Infof("There are no identity providers configured for cluster '%s'", clusterKey)
		os.Exit(0)
	}

	// Create the writer that will be used to print the tabulated results:
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if len(idps) == 1 && ocm.IdentityProviderType(idps[0]) == ocm.HTPasswdIDPType {
		fmt.Fprintf(writer, "NAME\t\tTYPE\n")
	} else {
		fmt.Fprintf(writer, "NAME\t\tTYPE\t\tAUTH URL\n")
	}
	for _, idp := range idps {
		idpType := ocm.IdentityProviderType(idp)
		fmt.Fprintf(writer, "%s\t\t%s\t\t%s\n", idp.Name(), idpType, getAuthURL(cluster, idp.Name(), idpType))
	}
	writer.Flush()
}

func getAuthURL(cluster *cmv1.Cluster, idpName, idpType string) string {
	if idpType == ocm.HTPasswdIDPType {
		return ""
	}
	oauthURL := strings.Replace(cluster.Console().URL(), "console-openshift-console", "oauth-openshift", 1)
	return fmt.Sprintf("%s/oauth2callback/%s", oauthURL, idpName)
}
