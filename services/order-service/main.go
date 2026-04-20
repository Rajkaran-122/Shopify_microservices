package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"digital-metro/ecomm_java/proto/order"
)

const port = ":50053"

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	orderSvc := &OrderServiceServer{}

	order.RegisterOrderServiceServer(s, orderSvc)
	fmt.Printf("Order service listening on %s\n", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

type OrderServiceServer struct {
	order.UnimplementedOrderServiceServer
}

func (s *OrderServiceServer) CreateOrder(ctx context.Context, req *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	return &order.CreateOrderResponse{
		Success: true,
		Message: "Order created",
		Order: &order.Order{
			Id:     "ord-123",
			UserId: req.UserId,
			Status: "pending",
		},
	}, nil
}

func (s *OrderServiceServer) GetOrder(ctx context.Context, req *order.GetOrderRequest) (*order.GetOrderResponse, error) {
	return &order.GetOrderResponse{
		Success: true,
		Order: &order.Order{
			Id:     req.OrderId,
			Status: "completed",
		},
	}, nil
}

func (s *OrderServiceServer) ListUserOrders(ctx context.Context, req *order.ListUserOrdersRequest) (*order.ListUserOrdersResponse, error) {
	return &order.ListUserOrdersResponse{
		Success: true,
		Total:   1,
		Orders: []*order.Order{
			{Id: "ord-123", UserId: req.UserId, Status: "completed"},
		},
	}, nil
}

func (s *OrderServiceServer) UpdateOrderStatus(ctx context.Context, req *order.UpdateOrderStatusRequest) (*order.UpdateOrderStatusResponse, error) {
	return &order.UpdateOrderStatusResponse{
		Success: true,
		Order: &order.Order{
			Id:     req.OrderId,
			Status: req.Status,
		},
	}, nil
}
