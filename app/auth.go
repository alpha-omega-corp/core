package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/alpha-omega-corp/core/app/models"
	"github.com/alpha-omega-corp/core/app/proto"
	"github.com/alpha-omega-corp/core/httputils"
	"github.com/uptrace/bun"
	"github.com/uptrace/bunrouter"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"net/http"
	"strings"
)

type AuthClient interface {
	Login(w http.ResponseWriter, req bunrouter.Request) error
	Validate(w http.ResponseWriter, req bunrouter.Request) error
	Register(w http.ResponseWriter, req bunrouter.Request) error
	GetUsers(w http.ResponseWriter, req bunrouter.Request) error
	CreateUser(w http.ResponseWriter, req bunrouter.Request) error
	UpdateUser(w http.ResponseWriter, req bunrouter.Request) error
	DeleteUser(w http.ResponseWriter, req bunrouter.Request) error
	AssignUser(w http.ResponseWriter, req bunrouter.Request) error
	GetUserPermissions(w http.ResponseWriter, req bunrouter.Request) error
	GetRoles(w http.ResponseWriter, req bunrouter.Request) error
	CreateRole(w http.ResponseWriter, req bunrouter.Request) error
	GetServices(w http.ResponseWriter, req bunrouter.Request) error
	GetServicePermissions(w http.ResponseWriter, req bunrouter.Request) error
	CreateServicePermissions(w http.ResponseWriter, req bunrouter.Request) error
}

type AuthServer struct {
	proto.UnimplementedAuthServiceServer

	db *bun.DB
	aw *AuthWrapper
}

func NewAuthServer(db *bun.DB, aw *AuthWrapper) *AuthServer {
	return &AuthServer{
		db: db,
		aw: aw,
	}
}

func RegisterAuthClient(client AuthClient, r *bunrouter.Router) AuthClient {
	r.GET("/users", client.GetUsers)
	r.POST("/users", client.CreateUser)
	r.PUT("/users/:id", client.UpdateUser)
	r.DELETE("/users/:id", client.DeleteUser)
	r.POST("/users/roles", client.AssignUser)
	r.GET("/users/:id/permissions", client.GetUserPermissions)
	r.GET("/auth/roles", client.GetRoles)
	r.POST("/auth/roles", client.CreateRole)
	r.GET("/auth/services", client.GetServices)
	r.GET("/auth/services/:id/permissions", client.GetServicePermissions)
	r.POST("/auth/services/permissions", client.CreateServicePermissions)
	r.POST("/auth/login", client.Login)
	r.POST("/auth/register", client.Register)
	r.POST("/auth/validate", client.Validate)

	return client
}

type authClient struct {
	AuthClient
	service proto.AuthServiceClient
}

func NewAuthClient(c *Config) AuthClient {
	conn, err := grpc.NewClient(*c.Url, grpc.WithInsecure())

	if err != nil {
		fmt.Println("Could not connect:", err)
	}

	return &authClient{service: proto.NewAuthServiceClient(conn)}
}

func (c *authClient) Login(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.LoginResponse, error) {
		data := httputils.GetBody[proto.LoginRequest](w, req)

		return c.service.Login(req.Context(), data)
	})
}

func (c *authClient) Validate(w http.ResponseWriter, req bunrouter.Request) error {
	authHeader := req.Header.Get("Authorization")
	token := strings.Split(authHeader, "Bearer ")[1]

	return httputils.Response(w, func() (*proto.ValidateResponse, error) {
		return c.service.Validate(req.Context(), &proto.ValidateRequest{
			Token: token,
		})
	})
}

func (c *authClient) Register(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.RegisterResponse, error) {
		data := httputils.GetBody[proto.RegisterRequest](w, req)

		return c.service.Register(req.Context(), data)
	})
}

func (c *authClient) GetUsers(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response[proto.GetUsersResponse](w, func() (*proto.GetUsersResponse, error) {
		return c.service.GetUsers(req.Context(), &emptypb.Empty{})
	})
}

func (c *authClient) GetRoles(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.GetRolesResponse, error) {
		return c.service.GetRoles(req.Context(), &emptypb.Empty{})
	})
}

func (c *authClient) GetServices(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.GetServicesResponse, error) {
		return c.service.GetServices(req.Context(), &emptypb.Empty{})
	})
}

func (c *authClient) CreateUser(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.CreateUserResponse, error) {
		data := httputils.GetBody[proto.CreateUserRequest](w, req)

		return c.service.CreateUser(req.Context(), data)
	})
}
func (c *authClient) UpdateUser(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.UpdateUserResponse, error) {
		data := httputils.GetBody[proto.UpdateUserRequest](w, req)

		return c.service.UpdateUser(req.Context(), data)
	})
}

func (c *authClient) DeleteUser(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.DeleteUserResponse, error) {
		data := httputils.GetBody[proto.DeleteUserRequest](w, req)
		return c.service.DeleteUser(req.Context(), data)
	})
}

func (c *authClient) CreateRole(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.CreateRoleResponse, error) {
		data := httputils.GetBody[proto.CreateRoleRequest](w, req)
		return c.service.CreateRole(req.Context(), data)
	})
}

func (c *authClient) AssignUser(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.AssignRoleResponse, error) {
		data := httputils.GetBody[proto.AssignRoleRequest](w, req)

		return c.service.AssignRole(req.Context(), data)
	})
}

func (c *authClient) CreatePermission(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.CreateServicePermissionsResponse, error) {
		data := httputils.GetBody[proto.CreateServicePermissionsRequest](w, req)

		return c.service.CreateServicePermissions(req.Context(), data)
	})
}

func (c *authClient) GetUserPermissions(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.GetUserPermissionsResponse, error) {
		data := httputils.GetBody[proto.GetUserPermissionsRequest](w, req)

		return c.service.GetUserPermissions(req.Context(), data)
	})
}

func (c *authClient) GetServicePermissions(w http.ResponseWriter, req bunrouter.Request) error {
	return httputils.Response(w, func() (*proto.GetServicePermissionsResponse, error) {
		data := httputils.GetParams[proto.GetServicePermissionsRequest](w, req)

		return c.service.GetServicePermissions(req.Context(), data)
	})
}

func (s *AuthServer) GetUser(ctx context.Context, req *proto.GetUserRequest) (*proto.GetUserResponse, error) {
	user := new(models.User)

	err := s.db.NewSelect().Model(&user).Relation("Roles").Where("id = ?", req.Id).Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &proto.GetUserResponse{
		User: &proto.User{},
	}, nil
}

func (s *AuthServer) GetUsers(ctx context.Context, _ *emptypb.Empty) (*proto.GetUsersResponse, error) {
	var users []*models.User

	err := s.db.NewSelect().Model(&users).Relation("Roles").Scan(ctx)
	if err != nil {
		return nil, err
	}

	var resSlice []*proto.User
	for _, user := range users {
		rolesSlice := make([]*proto.Role, len(user.Roles))

		for index, role := range user.Roles {
			rolesSlice[index] = &proto.Role{
				Id:   role.Id,
				Name: role.Name,
			}
		}

		resSlice = append(resSlice, &proto.User{
			Id:    user.Id,
			Name:  user.Name,
			Email: user.Email,
			Roles: rolesSlice,
		})
	}

	return &proto.GetUsersResponse{
		Users: resSlice,
	}, nil
}

func (s *AuthServer) CreateUser(ctx context.Context, req *proto.CreateUserRequest) (*proto.CreateUserResponse, error) {
	_, err := s.db.NewInsert().Model(&models.User{
		Name:  req.Name,
		Email: req.Email,
	}).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return &proto.CreateUserResponse{
		Status: http.StatusCreated,
	}, nil
}

func (s *AuthServer) UpdateUser(ctx context.Context, req *proto.UpdateUserRequest) (*proto.UpdateUserResponse, error) {
	if err := s.db.NewSelect().Model(&models.User{
		Name: req.Name,
	}).
		Where("id = ?", req.Id).Scan(ctx); err != nil {
		return nil, err
	}

	return &proto.UpdateUserResponse{
		Status: http.StatusOK,
	}, nil
}

func (s *AuthServer) DeleteUser(ctx context.Context, req *proto.DeleteUserRequest) (*proto.DeleteUserResponse, error) {
	_, err := s.db.NewDelete().Model(&models.User{}).Where("id = ?", req.Id).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return &proto.DeleteUserResponse{
		Status: http.StatusOK,
	}, nil
}

func (s *AuthServer) GetUserPermissions(ctx context.Context, req *proto.GetUserPermissionsRequest) (*proto.GetUserPermissionsResponse, error) {
	user := new(models.User)
	if err := s.db.NewSelect().
		Model(user).
		Relation("Roles").
		Where("id = ?", req.UserId).
		Scan(ctx); err != nil {
		return nil, err
	}

	var permSlice []models.Permission
	for _, role := range user.Roles {
		if err := s.db.NewSelect().
			Model(&role).
			Relation("Permissions").
			Where("id = ?", role.Id).
			Scan(ctx); err != nil {
			return nil, err
		}

		permSlice = append(permSlice, role.Permissions...)
	}

	permMap := make(map[string]bool)
	for index, perm := range permSlice {
		service := new(models.Service)
		if err := s.db.NewSelect().
			Model(service).
			Where("id = ?", perm.ServiceID).
			Scan(ctx); err != nil {
			return nil, err
		}

		svc := strings.ToLower(service.Name)
		idxRead := fmt.Sprintf("%s.read", svc)
		idxWrite := fmt.Sprintf("%s.write", svc)
		idxManage := fmt.Sprintf("%s.manage", svc)

		if index > 0 {
			if permMap[idxRead] != true {
				permMap[idxRead] = perm.Read
			}
			if permMap[idxWrite] != true {
				permMap[idxWrite] = perm.Write
			}
			if permMap[idxManage] != true {
				permMap[idxManage] = perm.Manage
			}
		} else {
			permMap[idxRead] = perm.Read
			permMap[idxWrite] = perm.Write
			permMap[idxManage] = perm.Manage
		}
	}

	return &proto.GetUserPermissionsResponse{
		Matrix: permMap,
	}, nil
}

func (s *AuthServer) GetRoles(ctx context.Context, _ *emptypb.Empty) (*proto.GetRolesResponse, error) {
	var roles []*models.Role

	err := s.db.NewSelect().Model(&roles).Scan(ctx)
	if err != nil {
		return nil, err
	}

	var resSlice []*proto.Role
	for _, role := range roles {
		resSlice = append(resSlice, &proto.Role{
			Id:   role.Id,
			Name: role.Name,
		})
	}

	return &proto.GetRolesResponse{
		Roles: resSlice,
	}, nil
}

func (s *AuthServer) CreateRole(ctx context.Context, req *proto.CreateRoleRequest) (*proto.CreateRoleResponse, error) {
	role := new(models.Role)
	role.Name = req.Name

	_, err := s.db.NewInsert().Model(role).Exec(ctx)

	if err != nil {
		return nil, err
	}

	return &proto.CreateRoleResponse{
		Status: http.StatusCreated,
	}, nil
}

func (s *AuthServer) AssignRole(ctx context.Context, req *proto.AssignRoleRequest) (*proto.AssignRoleResponse, error) {
	userRoles := new([]models.UserToRole)

	if err := s.db.NewSelect().Model(userRoles).Where("user_id = ?", req.UserId).Scan(ctx); err != nil {
		return nil, err
	}

	requestRoles := make(map[int64]int64, len(req.Roles))
	currentRoles := make(map[int64]int64, len(*userRoles))

	for idx, userRole := range *userRoles {
		currentRoles[userRole.RoleID] = int64(idx)
	}

	for idx, reqRole := range req.Roles {
		requestRoles[reqRole] = int64(idx)
	}

	// Add roles that are in the request
	for _, roleId := range req.Roles {
		if _, ok := currentRoles[roleId]; !ok {
			_, err := s.db.NewInsert().Model(&models.UserToRole{
				UserID: req.UserId,
				RoleID: roleId,
			}).Exec(ctx)

			if err != nil {
				return nil, err
			}
		}
	}

	// Delete user's roles that are not in the request
	for roleId := range currentRoles {
		if _, ok := requestRoles[roleId]; !ok {
			_, err := s.db.NewDelete().Model(&models.UserToRole{}).
				Where("user_id = ?", req.UserId).
				Where("role_id = ?", roleId).
				Exec(ctx)

			if err != nil {
				return nil, err
			}
		}
	}

	return &proto.AssignRoleResponse{
		Status: http.StatusCreated,
	}, nil
}

func (s *AuthServer) GetServices(ctx context.Context, _ *emptypb.Empty) (*proto.GetServicesResponse, error) {
	var services []models.Service
	if err := s.db.NewSelect().Model(&services).Scan(ctx); err != nil {
		return nil, err
	}

	var resSlice []*proto.Service
	for _, service := range services {
		resSlice = append(resSlice, &proto.Service{
			Id:   service.Id,
			Name: service.Name,
		})
	}

	return &proto.GetServicesResponse{
		Services: resSlice,
	}, nil
}
func (s *AuthServer) GetServicePermissions(ctx context.Context, req *proto.GetServicePermissionsRequest) (*proto.GetServicePermissionsResponse, error) {
	var service models.Service
	if err := s.db.NewSelect().
		Model(&service).
		Relation("Permissions").
		Where("id = ?", req.ServiceId).
		Scan(ctx); err != nil {
		return nil, err
	}

	resSlice := make([]*proto.Permission, len(service.Permissions))
	for index, permission := range service.Permissions {
		role := new(models.Role)
		if err := s.db.NewSelect().
			Model(role).
			Where("id  = ?", permission.RoleId).
			Scan(ctx); err != nil {
			return nil, err
		}

		resSlice[index] = &proto.Permission{
			Id: permission.Id,
			Service: &proto.Service{
				Id:   service.Id,
				Name: service.Name,
			},
			Role: &proto.Role{
				Id:   role.Id,
				Name: role.Name,
			},
			CanRead:   permission.Read,
			CanWrite:  permission.Write,
			CanManage: permission.Manage,
		}
	}

	return &proto.GetServicePermissionsResponse{
		Permissions: resSlice,
	}, nil
}
func (s *AuthServer) CreateServicePermissions(ctx context.Context, req *proto.CreateServicePermissionsRequest) (*proto.CreateServicePermissionsResponse, error) {
	permissions := &models.Permission{
		Read:      req.CanRead,
		Write:     req.CanWrite,
		Manage:    req.CanManage,
		ServiceID: req.ServiceId,
		RoleId:    req.RoleId,
	}

	_, err := s.db.NewInsert().Model(permissions).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return &proto.CreateServicePermissionsResponse{
		Status: http.StatusCreated,
	}, nil
}

func (s *AuthServer) Login(ctx context.Context, req *proto.LoginRequest) (*proto.LoginResponse, error) {
	var user models.User

	if err := s.db.
		NewSelect().
		Model(&user).
		Where("email = ?", req.Email).
		Scan(ctx, &user); err != nil {
		return nil, err
	}

	match := CheckPasswordHash(req.Password, user.Password)

	if !match {
		return nil, errors.New("invalid")
	}

	token, err := s.aw.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	return &proto.LoginResponse{
		Token: token,
		User: &proto.User{
			Id:    user.Id,
			Email: user.Email,
		},
	}, nil
}
func (s *AuthServer) Register(ctx context.Context, req *proto.RegisterRequest) (*proto.RegisterResponse, error) {
	_, err := s.db.NewInsert().Model(&models.User{
		Name:     req.Username,
		Email:    req.Email,
		Password: HashPassword(req.Password),
	}).Exec(ctx)

	if err != nil {
		return nil, err
	}

	return &proto.RegisterResponse{
		Status: http.StatusCreated,
	}, nil
}
func (s *AuthServer) Validate(ctx context.Context, req *proto.ValidateRequest) (*proto.ValidateResponse, error) {
	claims, err := s.aw.ValidateToken(req.Token)
	if err != nil {
		return nil, err
	}

	var user models.User
	err = s.db.NewSelect().Model(&user).Where("email = ?", claims.Email).Scan(ctx, &user)
	if err != nil {
		return nil, err
	}

	return &proto.ValidateResponse{
		User: &proto.User{
			Id:    user.Id,
			Email: user.Email,
		},
	}, nil
}
