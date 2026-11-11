// hello-connector is the smallest connector on the v1alpha1 contract: it lists the apps of a fictional
// platform through the host, reports each as a non-human identity, and reports a finding for one with
// no owner. It holds no credential and opens no connection except to the host that started it.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/ai-automation-tools/ai-security-command-center/src/spikes/pluginhost/pluginv1alpha1"
	"github.com/ai-automation-tools/ai-security-command-center/src/spikes/pluginhost/sdk"
)

type hello struct{}

func (hello) ID() string                            { return "hello-connector" }
func (hello) Version() string                       { return "0.0.1" }
func (hello) Health(context.Context) (bool, string) { return true, "ready" }

func (hello) Sync(ctx context.Context, host pb.HostClient, cursor string) (*pb.SyncResponse, error) {
	res, err := host.PlatformCall(ctx, &pb.PlatformCallRequest{CredentialKind: "example_app", Method: "GET", Url: "https://api.example.com/apps"})
	if err != nil {
		return nil, err
	}
	var apps []struct{ ID, Name, Owner string }
	if err := json.Unmarshal(res.GetBody(), &apps); err != nil {
		return nil, status.Error(codes.DataLoss, "the platform's answer wasn't the app list")
	}
	out := &pb.SyncResponse{Cursor: fmt.Sprint(len(apps))}
	// A refused call is counted and the sync goes on: one missing scope shouldn't lose the rest.
	count := func(err error) bool {
		if status.Code(err) == codes.PermissionDenied {
			out.Refused++
		}
		return err == nil
	}
	for _, a := range apps {
		_, err := host.UpsertNhi(ctx, &pb.UpsertNhiRequest{Nhi: &pb.Nhi{ExternalId: a.ID, Kind: "oauth_app", DisplayName: a.Name, Owner: a.Owner}})
		if count(err) {
			out.Upserted++
		}
		if a.Owner == "" {
			_, err := host.ReportFinding(ctx, &pb.ReportFindingRequest{RuleId: "nhi-no-owner", Subject: a.ID, Message: "this app has no owner"})
			count(err)
		}
	}
	return out, nil
}

func main() {
	if err := sdk.Serve(hello{}); err != nil {
		fmt.Fprintln(os.Stderr, "hello-connector:", err)
		os.Exit(1)
	}
}
