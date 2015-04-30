/*
Copyright 2014 Google Inc. All rights reserved.

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

package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

const (
	delete_long = `Delete a resource by filename, stdin, resource and ID, or by resources and label selector.

JSON and YAML formats are accepted.

If both a filename and command line arguments are passed, the command line
arguments are used and the filename is ignored.

Note that the delete command does NOT do resource version checks, so if someone
submits an update to a resource right when you submit a delete, their update
will be lost along with the rest of the resource.`
	delete_example = `// Delete a pod using the type and ID specified in pod.json.
$ kubectl delete -f pod.json

// Delete a pod based on the type and ID in the JSON passed into stdin.
$ cat pod.json | kubectl delete -f -

// Delete pods and services with label name=myLabel.
$ kubectl delete pods,services -l name=myLabel

// Delete a pod with ID 1234-56-7890-234234-456456.
$ kubectl delete pod 1234-56-7890-234234-456456

// Delete all pods
$ kubectl delete pods --all`
)

func NewCmdDelete(f *cmdutil.Factory, out io.Writer) *cobra.Command {
	var filenames util.StringList
	cmd := &cobra.Command{
		Use:     "delete ([-f FILENAME] | (RESOURCE [(ID | -l label | --all)]",
		Short:   "Delete a resource by filename, stdin, resource and ID, or by resources and label selector.",
		Long:    delete_long,
		Example: delete_example,
		Run: func(cmd *cobra.Command, args []string) {
			err := RunDelete(f, out, cmd, args, filenames)
			cmdutil.CheckErr(err)
		},
	}
	usage := "Filename, directory, or URL to a file containing the resource to delete"
	kubectl.AddJsonFilenameFlag(cmd, &filenames, usage)
	cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().Bool("all", false, "[-all] to select all the specified resources")
	cmd.Flags().Bool("cascade", true, "If true, cascade the delete resources managed by this resource (e.g. Pods created by a ReplicationController).  Default true.")
	return cmd
}

func RunDelete(f *cmdutil.Factory, out io.Writer, cmd *cobra.Command, args []string, filenames util.StringList) error {
	cmdNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	mapper, typer := f.Object()
	r := resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(filenames...).
		SelectorParam(cmdutil.GetFlagString(cmd, "selector")).
		SelectAllParam(cmdutil.GetFlagBool(cmd, "all")).
		ResourceTypeOrNameArgs(false, args...).RequireObject(false).
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}

	// By default use a reaper to delete all related resources.
	if cmdutil.GetFlagBool(cmd, "cascade") {
		return ReapResult(r, f, out, cmdutil.GetFlagBool(cmd, "cascade"))
	}
	return DeleteResult(r, out)
}

func ReapResult(r *resource.Result, f *cmdutil.Factory, out io.Writer, isDefaultDelete bool) error {
	found := 0
	err := r.IgnoreErrors(errors.IsNotFound).Visit(func(info *resource.Info) error {
		found++
		reaper, err := f.Reaper(info.Mapping)
		if err != nil {
			// If the error is "not found" and the user didn't explicitly ask for stop.
			if kubectl.IsNoSuchReaperError(err) && isDefaultDelete {
				return deleteResource(info, out)
			}
			return err
		}
		if _, err := reaper.Stop(info.Namespace, info.Name); err != nil {
			return err
		}
		fmt.Fprintf(out, "%s/%s\n", info.Mapping.Resource, info.Name)
		return nil
	})
	if err != nil {
		return err
	}
	if found == 0 {
		fmt.Fprintf(out, "No resources found\n")
	}
	return nil
}

func DeleteResult(r *resource.Result, out io.Writer) error {
	found := 0
	err := r.IgnoreErrors(errors.IsNotFound).Visit(func(info *resource.Info) error {
		found++
		return deleteResource(info, out)
	})
	if err != nil {
		return err
	}
	if found == 0 {
		fmt.Fprintf(out, "No resources found\n")
	}
	return nil
}

func deleteResource(info *resource.Info, out io.Writer) error {
	if err := resource.NewHelper(info.Client, info.Mapping).Delete(info.Namespace, info.Name); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s/%s\n", info.Mapping.Resource, info.Name)
	return nil
}