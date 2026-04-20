package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"digital-metro/ecomm_java/proto/product"
)

const port = ":50052"

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	productSvc := &ProductServiceServer{}

	if err := productSvc.InitServices(); err != nil {
		log.Fatalf("failed to init: %v", err)
	}

	defer productSvc.Close()

	product.RegisterProductServiceServer(s, productSvc)

	fmt.Printf("Product service listening on %s\n", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

type ProductServiceServer struct {
	redis *redis.Client
	product.UnimplementedProductServiceServer
}

func (s *ProductServiceServer) InitServices() error {
	s.redis = redis.NewClient(&redis.Options{Addr: "redis:6379"})
	fmt.Println("Product service initialized")
	return nil
}

func (s *ProductServiceServer) Close() error {
	if s.redis != nil {
		return s.redis.Close()
	}
	return nil
}

func (s *ProductServiceServer) ListProducts(ctx context.Context, req *product.ListProductsRequest) (*product.ListProductsResponse, error) {
	return &product.ListProductsResponse{
		Success: true,
		Total:   2,
		Products: []*product.Product{
			{Id: "p1", Name: "Product 1", Price: 99.99, Quantity: 100},
			{Id: "p2", Name: "Product 2", Price: 199.99, Quantity: 50},
		},
	}, nil
}

func (s *ProductServiceServer) GetProduct(ctx context.Context, req *product.GetProductRequest) (*product.GetProductResponse, error) {
	return &product.GetProductResponse{
		Success: true,
		Product: &product.Product{
			Id:       req.ProductId,
			Name:     "Sample Product",
			Price:    99.99,
			Quantity: 100,
		},
	}, nil
}

func (s *ProductServiceServer) CreateProduct(ctx context.Context, req *product.CreateProductRequest) (*product.CreateProductResponse, error) {
	return &product.CreateProductResponse{
		Success: true,
		Message: "Product created",
		Product: &product.Product{
			Id:       "p-new",
			Name:     req.Name,
			Price:    req.Price,
			Quantity: req.Quantity,
		},
	}, nil
}

func (s *ProductServiceServer) UpdateProductInventory(ctx context.Context, req *product.UpdateInventoryRequest) (*product.UpdateInventoryResponse, error) {
	return &product.UpdateInventoryResponse{
		Success:     true,
		NewQuantity: 95,
	}, nil
}

func (s *ProductServiceServer) CheckInventory(ctx context.Context, req *product.CheckInventoryRequest) (*product.CheckInventoryResponse, error) {
	return &product.CheckInventoryResponse{
		Available:       true,
		CurrentQuantity: 100,
	}, nil
}
