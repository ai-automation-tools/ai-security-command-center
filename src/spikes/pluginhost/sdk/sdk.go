// Package sdk is the Go SDK skeleton for the v1alpha1 plugin contract: a connector implements
// Connector and calls Serve from main. It is a spike; helpers for cursors, retries and rate limits
// (FR-PLG-08) aren't here yet.
package sdk

import (
	"bufio"
	"context"
	"errors"
	"net"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "github.com/ai-automation-tools/ai-security-command-center/src/spikes/pluginhost/pluginv1alpha1"
)

// ContractVersion is the contract this SDK speaks.
const ContractVersion = "v1alpha1"

// Connector is what a connector plugin implements.
type Connector interface {
	// ID must be the id the plugin was approved under: the host stops a plugin that says otherwise.
	// Version is reported as it is; the prototype host doesn't check it.
	ID() string
	Version() string
	Health(ctx context.Context) (ok bool, detail string)
	// Sync reads the platform through host.PlatformCall and reports through host.UpsertNhi and
	// host.ReportFinding. The plugin holds no credential and opens no connection of its own.
	Sync(ctx context.Context, host pb.HostClient, cursor string) (*pb.SyncResponse, error)
}

// Serve runs the connector under the plugin host that started this process. It returns only on error.
func Serve(c Connector) error {
	hostSock, pluginSock := os.Getenv("ASCC_HOST_SOCKET"), os.Getenv("ASCC_PLUGIN_SOCKET")
	if hostSock == "" || pluginSock == "" {
		return errors.New("this is an ASCC plugin: it runs under the plugin host, not on its own")
	}
	token, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return errors.New("the plugin host gave no capability token")
	}
	bearer := "Bearer " + strings.TrimSpace(token)
	conn, err := grpc.NewClient("passthrough:///host",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", hostSock)
		}),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(metadata.AppendToOutgoingContext(ctx, "authorization", bearer), method, req, reply, cc, opts...)
		}))
	if err != nil {
		return err
	}
	lis, err := net.Listen("unix", pluginSock)
	if err != nil {
		return err
	}
	srv := grpc.NewServer()
	s := &server{c: c, host: pb.NewHostClient(conn)}
	pb.RegisterPluginServer(srv, s)
	pb.RegisterConnectorServer(srv, s)
	return srv.Serve(lis)
}

type server struct {
	pb.UnimplementedPluginServer
	pb.UnimplementedConnectorServer
	c    Connector
	host pb.HostClient
}

func (s *server) Describe(context.Context, *pb.DescribeRequest) (*pb.DescribeResponse, error) {
	return &pb.DescribeResponse{PluginId: s.c.ID(), Version: s.c.Version(), Type: "connector", ContractVersion: ContractVersion}, nil
}

func (s *server) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	ok, detail := s.c.Health(ctx)
	return &pb.HealthResponse{Ok: ok, Detail: detail}, nil
}

func (s *server) Sync(ctx context.Context, r *pb.SyncRequest) (*pb.SyncResponse, error) {
	return s.c.Sync(ctx, s.host, r.GetCursor())
}
