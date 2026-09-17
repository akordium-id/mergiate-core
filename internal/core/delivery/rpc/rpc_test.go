package rpc_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	mergiatev1 "github.com/akordium-id/mergiate-core/gen/mergiate/v1"
	"github.com/akordium-id/mergiate-core/gen/mergiate/v1/mergiatev1connect"
	deliveryrpc "github.com/akordium-id/mergiate-core/internal/core/delivery/rpc"
	"github.com/akordium-id/mergiate-core/internal/core/domain/contact"
	domainparty "github.com/akordium-id/mergiate-core/internal/core/domain/party"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	partyuc "github.com/akordium-id/mergiate-core/internal/core/usecase/party"
	"github.com/akordium-id/mergiate-core/pkg/auth"
)

// mockPartyUsecase implements partyuc.Usecase for testing.
type mockPartyUsecase struct {
	parties map[shared.ID]*domainparty.Party
}

func newMockPartyUsecase() *mockPartyUsecase {
	return &mockPartyUsecase{
		parties: make(map[shared.ID]*domainparty.Party),
	}
}

func (m *mockPartyUsecase) CreateParty(ctx context.Context, cmd partyuc.CreatePartyCommand) (*domainparty.Party, error) {
	tenantID, err := shared.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	id, _ := shared.NewID()
	p := &domainparty.Party{
		ID:        id,
		TenantID:  tenantID,
		Type:      cmd.Type,
		Code:      cmd.Code,
		Name:      cmd.Name,
		LegalName: cmd.LegalName,
		TaxID:     cmd.TaxID,
		Status:    domainparty.StatusActive,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	m.parties[id] = p
	return p, nil
}

func (m *mockPartyUsecase) GetParty(ctx context.Context, id shared.ID) (*partyuc.PartyDetail, error) {
	p, ok := m.parties[id]
	if !ok {
		return nil, shared.ErrNotFound
	}
	return &partyuc.PartyDetail{
		Party: *p,
	}, nil
}

func (m *mockPartyUsecase) ListParties(ctx context.Context, page, pageSize int32, partyType *domainparty.Type, roleType *domainparty.RoleType) (*partyuc.PartyListResult, error) {
	var items []domainparty.Party
	for _, p := range m.parties {
		items = append(items, *p)
	}
	return &partyuc.PartyListResult{
		Items:    items,
		Total:    int64(len(items)),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (m *mockPartyUsecase) UpdateParty(ctx context.Context, cmd partyuc.UpdatePartyCommand) (*domainparty.Party, error) {
	return nil, nil
}
func (m *mockPartyUsecase) AddRole(ctx context.Context, cmd partyuc.AddRoleCommand) (*domainparty.PartyRole, error) {
	return nil, nil
}
func (m *mockPartyUsecase) RemoveRole(ctx context.Context, roleID shared.ID) error {
	return nil
}
func (m *mockPartyUsecase) AddAddress(ctx context.Context, partyID shared.ID, cmd partyuc.AddAddressCommand) (*contact.Address, error) {
	return nil, nil
}
func (m *mockPartyUsecase) AddContact(ctx context.Context, partyID shared.ID, cmd partyuc.AddContactCommand) (*contact.Contact, error) {
	return nil, nil
}

func TestPingService_ConnectRPC(t *testing.T) {
	tokenMgr := auth.NewTokenManager("test-secret", "test-app")
	mockUC := newMockPartyUsecase()

	rpcServer := deliveryrpc.NewServer(deliveryrpc.Config{
		TokenManager: tokenMgr,
		PartyUsecase: mockUC,
		AppVersion:   "1.0.0-test",
	})

	ts := httptest.NewServer(rpcServer.Handler())
	defer ts.Close()

	client := mergiatev1connect.NewPingServiceClient(http.DefaultClient, ts.URL)

	res, err := client.Ping(context.Background(), connect.NewRequest(&mergiatev1.PingRequest{
		Message: "hello mergiate",
	}))
	require.NoError(t, err)
	assert.Equal(t, "hello mergiate", res.Msg.Message)
	assert.Equal(t, "1.0.0-test", res.Msg.Version)
	assert.True(t, res.Msg.Timestamp > 0)
}

func TestPartyService_ConnectRPC_AuthRequired(t *testing.T) {
	tokenMgr := auth.NewTokenManager("test-secret-32-bytes-long-super-sec!", "test-app")
	mockUC := newMockPartyUsecase()

	rpcServer := deliveryrpc.NewServer(deliveryrpc.Config{
		TokenManager: tokenMgr,
		PartyUsecase: mockUC,
		AppVersion:   "1.0.0-test",
	})

	ts := httptest.NewServer(rpcServer.Handler())
	defer ts.Close()

	client := mergiatev1connect.NewPartyServiceClient(http.DefaultClient, ts.URL)

	// 1. Unauthenticated request should fail
	_, err := client.CreateParty(context.Background(), connect.NewRequest(&mergiatev1.CreatePartyRequest{
		Name: "Acme Corp",
	}))
	require.Error(t, err)
	connectErr, ok := err.(*connect.Error)
	require.True(t, ok)
	assert.Equal(t, connect.CodeUnauthenticated, connectErr.Code())

	// 2. Authenticated request with valid JWT
	tenantID, _ := shared.NewID()
	userID, _ := shared.NewID()
	token, err := tokenMgr.GenerateToken(
		userID,
		tenantID,
		"admin@acme.com",
		"Admin User",
		[]string{"admin"},
		[]string{"party:create", "party:read"},
		time.Hour,
	)
	require.NoError(t, err)

	req := connect.NewRequest(&mergiatev1.CreatePartyRequest{
		Type: "organization",
		Name: "Acme Corp",
		Code: "ACM",
	})
	req.Header().Set("Authorization", "Bearer "+token)

	res, err := client.CreateParty(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "Acme Corp", res.Msg.Party.Name)
	assert.Equal(t, "ACM", res.Msg.Party.Code)
	assert.Equal(t, tenantID.String(), res.Msg.Party.TenantId)

	// 3. Get party by ID
	getReq := connect.NewRequest(&mergiatev1.GetPartyRequest{
		Id: res.Msg.Party.Id,
	})
	getReq.Header().Set("Authorization", "Bearer "+token)

	getRes, err := client.GetParty(context.Background(), getReq)
	require.NoError(t, err)
	assert.Equal(t, "Acme Corp", getRes.Msg.Party.Name)
}

func TestPingService_PureGRPC(t *testing.T) {
	tokenMgr := auth.NewTokenManager("test-secret", "test-app")
	mockUC := newMockPartyUsecase()

	rpcServer := deliveryrpc.NewServer(deliveryrpc.Config{
		TokenManager: tokenMgr,
		PartyUsecase: mockUC,
		AppVersion:   "1.0.0-test",
	})

	grpcServer := rpcServer.BuildGRPCServer()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()

	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.GracefulStop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client := mergiatev1.NewPingServiceClient(conn)

	res, err := client.Ping(context.Background(), &mergiatev1.PingRequest{
		Message: "pure grpc ping",
	})
	require.NoError(t, err)
	assert.Equal(t, "pure grpc ping", res.Message)
	assert.Equal(t, "1.0.0-test", res.Version)
}

