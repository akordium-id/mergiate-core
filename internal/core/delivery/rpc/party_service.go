package rpc

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mergiatev1 "github.com/akordium-id/mergiate-core/gen/mergiate/v1"
	"github.com/akordium-id/mergiate-core/gen/mergiate/v1/mergiatev1connect"
	domainparty "github.com/akordium-id/mergiate-core/internal/core/domain/party"
	"github.com/akordium-id/mergiate-core/internal/core/domain/shared"
	partyuc "github.com/akordium-id/mergiate-core/internal/core/usecase/party"
)

// PartyServiceServer handles Party operations via ConnectRPC and gRPC.
type PartyServiceServer struct {
	mergiatev1.UnimplementedPartyServiceServer
	uc partyuc.Usecase
}

func NewPartyServiceServer(uc partyuc.Usecase) *PartyServiceServer {
	return &PartyServiceServer{uc: uc}
}

// ConnectRPC Methods

func (s *PartyServiceServer) GetParty(
	ctx context.Context,
	req *connect.Request[mergiatev1.GetPartyRequest],
) (*connect.Response[mergiatev1.GetPartyResponse], error) {
	resp, err := s.getPartyCore(ctx, req.Msg)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *PartyServiceServer) CreateParty(
	ctx context.Context,
	req *connect.Request[mergiatev1.CreatePartyRequest],
) (*connect.Response[mergiatev1.CreatePartyResponse], error) {
	resp, err := s.createPartyCore(ctx, req.Msg)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(resp), nil
}

func (s *PartyServiceServer) ListParties(
	ctx context.Context,
	req *connect.Request[mergiatev1.ListPartiesRequest],
) (*connect.Response[mergiatev1.ListPartiesResponse], error) {
	resp, err := s.listPartiesCore(ctx, req.Msg)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(resp), nil
}

// Pure gRPC Methods

type grpcPartyAdapter struct {
	mergiatev1.UnimplementedPartyServiceServer
	server *PartyServiceServer
}

func (g *grpcPartyAdapter) GetParty(ctx context.Context, req *mergiatev1.GetPartyRequest) (*mergiatev1.GetPartyResponse, error) {
	resp, err := g.server.getPartyCore(ctx, req)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return resp, nil
}

func (g *grpcPartyAdapter) CreateParty(ctx context.Context, req *mergiatev1.CreatePartyRequest) (*mergiatev1.CreatePartyResponse, error) {
	resp, err := g.server.createPartyCore(ctx, req)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return resp, nil
}

func (g *grpcPartyAdapter) ListParties(ctx context.Context, req *mergiatev1.ListPartiesRequest) (*mergiatev1.ListPartiesResponse, error) {
	resp, err := g.server.listPartiesCore(ctx, req)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return resp, nil
}

func (s *PartyServiceServer) GRPCServer() mergiatev1.PartyServiceServer {
	return &grpcPartyAdapter{server: s}
}

// Core Business Logic (Shared by Connect and gRPC)

func (s *PartyServiceServer) GetPartyCore(ctx context.Context, req *mergiatev1.GetPartyRequest) (*mergiatev1.GetPartyResponse, error) {
	return s.getPartyCore(ctx, req)
}

func (s *PartyServiceServer) getPartyCore(ctx context.Context, req *mergiatev1.GetPartyRequest) (*mergiatev1.GetPartyResponse, error) {
	id, err := shared.ParseID(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid party id: %v", err)
	}

	detail, err := s.uc.GetParty(ctx, id)
	if err != nil {
		return nil, err
	}

	pbParty := mapDomainPartyToProto(&detail.Party)
	var pbAddresses []*mergiatev1.ContactAddress
	for _, addr := range detail.Addresses {
		pbAddresses = append(pbAddresses, &mergiatev1.ContactAddress{
			Id:          addr.ID.String(),
			Type:        string(addr.Type),
			Label:       addr.Label,
			Line1:       addr.Line1,
			Line2:       addr.Line2,
			City:        addr.City,
			State:       addr.State,
			PostalCode:  addr.PostalCode,
			CountryCode: addr.CountryCode,
			IsPrimary:   addr.IsPrimary,
		})
	}

	var pbContacts []*mergiatev1.ContactItem
	for _, c := range detail.Contacts {
		pbContacts = append(pbContacts, &mergiatev1.ContactItem{
			Id:        c.ID.String(),
			Type:      string(c.Type),
			Value:     c.Value,
			Label:     c.Label,
			IsPrimary: c.IsPrimary,
		})
	}

	return &mergiatev1.GetPartyResponse{
		Party:     pbParty,
		Addresses: pbAddresses,
		Contacts:  pbContacts,
	}, nil
}

func (s *PartyServiceServer) createPartyCore(ctx context.Context, req *mergiatev1.CreatePartyRequest) (*mergiatev1.CreatePartyResponse, error) {
	var initialRoles []domainparty.RoleType
	for _, r := range req.GetInitialRoles() {
		initialRoles = append(initialRoles, domainparty.RoleType(r))
	}

	cmd := partyuc.CreatePartyCommand{
		Type:         domainparty.Type(req.GetType()),
		Code:         req.GetCode(),
		Name:         req.GetName(),
		LegalName:    req.GetLegalName(),
		TaxID:        req.GetTaxId(),
		InitialRoles: initialRoles,
	}

	party, err := s.uc.CreateParty(ctx, cmd)
	if err != nil {
		return nil, err
	}

	return &mergiatev1.CreatePartyResponse{
		Party: mapDomainPartyToProto(party),
	}, nil
}

func (s *PartyServiceServer) listPartiesCore(ctx context.Context, req *mergiatev1.ListPartiesRequest) (*mergiatev1.ListPartiesResponse, error) {
	page := req.GetPage()
	if page <= 0 {
		page = 1
	}
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 20
	}

	var partyType *domainparty.Type
	if pt := req.GetPartyType(); pt != "" {
		t := domainparty.Type(pt)
		partyType = &t
	}

	var roleType *domainparty.RoleType
	if rt := req.GetRoleType(); rt != "" {
		r := domainparty.RoleType(rt)
		roleType = &r
	}

	res, err := s.uc.ListParties(ctx, page, pageSize, partyType, roleType)
	if err != nil {
		return nil, err
	}

	var items []*mergiatev1.Party
	for _, item := range res.Items {
		items = append(items, mapDomainPartyToProto(&item))
	}

	return &mergiatev1.ListPartiesResponse{
		Items:    items,
		Total:    res.Total,
		Page:     res.Page,
		PageSize: res.PageSize,
	}, nil
}

func mapDomainPartyToProto(p *domainparty.Party) *mergiatev1.Party {
	if p == nil {
		return nil
	}

	var roles []*mergiatev1.PartyRole
	for _, r := range p.Roles {
		var orgID string
		if r.OrganizationID != nil {
			orgID = r.OrganizationID.String()
		}
		roles = append(roles, &mergiatev1.PartyRole{
			Id:             r.ID.String(),
			TenantId:       r.TenantID.String(),
			PartyId:        r.PartyID.String(),
			OrganizationId: orgID,
			RoleType:       string(r.RoleType),
			Status:         string(r.Status),
			CreatedAt:      r.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      r.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &mergiatev1.Party{
		Id:        p.ID.String(),
		TenantId:  p.TenantID.String(),
		Type:      string(p.Type),
		Code:      p.Code,
		Name:      p.Name,
		LegalName: p.LegalName,
		TaxId:     p.TaxID,
		Status:    string(p.Status),
		Roles:     roles,
		CreatedAt: p.CreatedAt.Format(time.RFC3339),
		UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
	}
}

func toConnectError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, shared.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, shared.ErrAlreadyExists):
		return connect.NewError(connect.CodeAlreadyExists, err)
	case errors.Is(err, shared.ErrForbidden), errors.Is(err, shared.ErrTenantSuspended):
		return connect.NewError(connect.CodePermissionDenied, err)
	case errors.Is(err, shared.ErrUnauthorized):
		return connect.NewError(connect.CodeUnauthenticated, err)
	case errors.Is(err, shared.ErrInvalidInput), errors.Is(err, shared.ErrTenantRequired):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, shared.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, shared.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, shared.ErrForbidden), errors.Is(err, shared.ErrTenantSuspended):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, shared.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, shared.ErrInvalidInput), errors.Is(err, shared.ErrTenantRequired):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

var _ mergiatev1connect.PartyServiceHandler = (*PartyServiceServer)(nil)
