package plugin

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"

	go_plugin "github.com/hashicorp/go-plugin"
	"github.com/hashicorp/vagrant/ext/go-plugin/vagrant"
	"github.com/hashicorp/vagrant/ext/go-plugin/vagrant/plugin/proto/vagrant_common"
	"github.com/hashicorp/vagrant/ext/go-plugin/vagrant/plugin/proto/vagrant_config"

	"github.com/LK4D4/joincontext"
)

type Config interface {
	vagrant.Config
	Meta
}

type ConfigPlugin struct {
	go_plugin.NetRPCUnsupportedPlugin
	Impl Config
}

func (c *ConfigPlugin) GRPCServer(broker *go_plugin.GRPCBroker, s *grpc.Server) error {
	c.Impl.Init()
	vagrant_config.RegisterConfigServer(s, &GRPCConfigServer{
		Impl: c.Impl,
		GRPCIOServer: GRPCIOServer{
			Impl: c.Impl}})
	return nil
}

func (c *ConfigPlugin) GRPCClient(ctx context.Context, broker *go_plugin.GRPCBroker, con *grpc.ClientConn) (interface{}, error) {
	client := vagrant_config.NewConfigClient(con)
	return &GRPCConfigClient{
		client:  client,
		doneCtx: ctx,
		GRPCIOClient: GRPCIOClient{
			client:  client,
			doneCtx: ctx}}, nil
}

type GRPCConfigServer struct {
	GRPCIOServer
	Impl Config
}

func (s *GRPCConfigServer) ConfigAttributes(ctx context.Context, req *vagrant_common.NullRequest) (resp *vagrant_config.AttributesResponse, err error) {
	resp = &vagrant_config.AttributesResponse{}
	n := make(chan struct{}, 1)
	go func() {
		resp.Attributes, err = s.Impl.ConfigAttributes()
		n <- struct{}{}
	}()
	select {
	case <-ctx.Done():
	case <-n:
	}
	return
}

func (s *GRPCConfigServer) ConfigLoad(ctx context.Context, req *vagrant_config.LoadRequest) (resp *vagrant_config.LoadResponse, err error) {
	resp = &vagrant_config.LoadResponse{}
	var data, r map[string]interface{}
	err = json.Unmarshal([]byte(req.Data), &data)
	if err != nil {
		return
	}
	n := make(chan struct{}, 1)
	go func() {
		r, err = s.Impl.ConfigLoad(ctx, data)
		n <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return
	case <-n:
	}

	if err != nil {
		return
	}
	mdata, err := json.Marshal(r)
	if err != nil {
		return
	}

	resp.Data = string(mdata)
	return
}

func (s *GRPCConfigServer) ConfigValidate(ctx context.Context, req *vagrant_config.ValidateRequest) (resp *vagrant_config.ValidateResponse, err error) {
	resp = &vagrant_config.ValidateResponse{}
	var data map[string]interface{}
	err = json.Unmarshal([]byte(req.Data), &data)
	if err != nil {
		return
	}
	m, err := vagrant.LoadMachine(req.Machine, s.Impl)
	if err != nil {
		return
	}
	n := make(chan struct{}, 1)
	go func() {
		resp.Errors, err = s.Impl.ConfigValidate(ctx, data, m)
		n <- struct{}{}
	}()

	select {
	case <-ctx.Done():
	case <-n:
	}

	return
}

func (s *GRPCConfigServer) ConfigFinalize(ctx context.Context, req *vagrant_config.FinalizeRequest) (resp *vagrant_config.FinalizeResponse, err error) {
	resp = &vagrant_config.FinalizeResponse{}
	var data, r map[string]interface{}
	err = json.Unmarshal([]byte(req.Data), &data)
	if err != nil {
		return
	}
	n := make(chan struct{}, 1)
	go func() {
		r, err = s.Impl.ConfigFinalize(ctx, data)
		n <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return
	case <-n:
	}

	if err != nil {
		return
	}
	mdata, err := json.Marshal(r)
	if err != nil {
		return
	}
	resp.Data = string(mdata)
	return
}

type GRPCConfigClient struct {
	GRPCCoreClient
	GRPCIOClient
	client  vagrant_config.ConfigClient
	doneCtx context.Context
}

func (c *GRPCConfigClient) ConfigAttributes() (attrs []string, err error) {
	ctx := context.Background()
	jctx, _ := joincontext.Join(ctx, c.doneCtx)
	resp, err := c.client.ConfigAttributes(jctx, &vagrant_common.NullRequest{})
	if err != nil {
		return nil, handleGrpcError(err, c.doneCtx, nil)
	}
	attrs = resp.Attributes
	return
}

func (c *GRPCConfigClient) ConfigLoad(ctx context.Context, data map[string]interface{}) (loaddata map[string]interface{}, err error) {
	mdata, err := json.Marshal(data)
	if err != nil {
		return
	}
	jctx, _ := joincontext.Join(ctx, c.doneCtx)
	resp, err := c.client.ConfigLoad(jctx, &vagrant_config.LoadRequest{
		Data: string(mdata)})
	if err != nil {
		return nil, handleGrpcError(err, c.doneCtx, ctx)
	}
	err = json.Unmarshal([]byte(resp.Data), &loaddata)
	return
}

func (c *GRPCConfigClient) ConfigValidate(ctx context.Context, data map[string]interface{}, m *vagrant.Machine) (errs []string, err error) {
	machData, err := vagrant.DumpMachine(m)
	if err != nil {
		return
	}
	mdata, err := json.Marshal(data)
	if err != nil {
		return
	}
	jctx, _ := joincontext.Join(ctx, c.doneCtx)
	resp, err := c.client.ConfigValidate(jctx, &vagrant_config.ValidateRequest{
		Data:    string(mdata),
		Machine: machData})
	if err != nil {
		return nil, handleGrpcError(err, c.doneCtx, ctx)
	}
	errs = resp.Errors
	return
}

func (c *GRPCConfigClient) ConfigFinalize(ctx context.Context, data map[string]interface{}) (finaldata map[string]interface{}, err error) {
	mdata, err := json.Marshal(data)
	if err != nil {
		return
	}
	jctx, _ := joincontext.Join(ctx, c.doneCtx)
	resp, err := c.client.ConfigFinalize(jctx, &vagrant_config.FinalizeRequest{
		Data: string(mdata)})
	if err != nil {
		return nil, handleGrpcError(err, c.doneCtx, ctx)
	}
	err = json.Unmarshal([]byte(resp.Data), &finaldata)
	return
}