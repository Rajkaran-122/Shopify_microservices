package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"digital-metro/ecomm_java/proto/notification"
)

const port = ":50054"

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	notifSvc := &NotificationServiceServer{}

	notification.RegisterNotificationServiceServer(s, notifSvc)
	fmt.Printf("Notification service listening on %s\n", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

type NotificationServiceServer struct {
	notification.UnimplementedNotificationServiceServer
}

func (s *NotificationServiceServer) SendOrderNotification(ctx context.Context, req *notification.OrderNotificationRequest) (*notification.NotificationResponse, error) {
	fmt.Printf("Sending notification to %s for order %s\n", req.UserEmail, req.OrderId)
	return &notification.NotificationResponse{
		Success:        true,
		Message:        "Notification sent",
		NotificationId: "notif-123",
	}, nil
}

func (s *NotificationServiceServer) SendEmailNotification(ctx context.Context, req *notification.EmailNotificationRequest) (*notification.NotificationResponse, error) {
	return &notification.NotificationResponse{
		Success:        true,
		Message:        "Email sent",
		NotificationId: "email-123",
	}, nil
}

func (s *NotificationServiceServer) GetNotificationHistory(ctx context.Context, req *notification.NotificationHistoryRequest) (*notification.NotificationHistoryResponse, error) {
	return &notification.NotificationHistoryResponse{
		Success: true,
		Total:   0,
	}, nil
}
