package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"google.golang.org/grpc"

	"digital-metro/ecomm_java/proto/user"
	"digital-metro/ecomm_java/proto/product"
	"digital-metro/ecomm_java/proto/order"
)

const (
	port = ":8080"
	userServiceAddr = "user-service:50051"
	productServiceAddr = "product-service:50052"
	orderServiceAddr = "order-service:50053"
)

type APIGateway struct {
	userClient user.UserServiceClient
	productClient product.ProductServiceClient
	orderClient order.OrderServiceClient
}

func main() {
	userConn, err := grpc.Dial(userServiceAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to connect user service: %v", err)
	}
	defer userConn.Close()

	productConn, err := grpc.Dial(productServiceAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to connect product service: %v", err)
	}
	defer productConn.Close()

	orderConn, err := grpc.Dial(orderServiceAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("failed to connect order service: %v", err)
	}
	defer orderConn.Close()

	gateway := &APIGateway{
		userClient:    user.NewUserServiceClient(userConn),
		productClient: product.NewProductServiceClient(productConn),
		orderClient:   order.NewOrderServiceClient(orderConn),
	}

	// Routes
	http.HandleFunc("/health", gateway.healthCheck)
	http.HandleFunc("/api/users/register", gateway.register)
	http.HandleFunc("/api/users/login", gateway.login)
	http.HandleFunc("/api/products", gateway.listProducts)
	http.HandleFunc("/api/orders", gateway.createOrder)

	fmt.Printf("API Gateway listening on %s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

func (g *APIGateway) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (g *APIGateway) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req user.RegisterRequest
	json.NewDecoder(r.Body).Decode(&req)

	resp, err := g.userClient.Register(context.Background(), &req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func (g *APIGateway) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req user.LoginRequest
	json.NewDecoder(r.Body).Decode(&req)

	resp, err := g.userClient.Login(context.Background(), &req)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func (g *APIGateway) listProducts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp, err := g.productClient.ListProducts(context.Background(), &product.ListProductsRequest{Page: 1, Limit: 10})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}

func (g *APIGateway) createOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var req order.CreateOrderRequest
	json.NewDecoder(r.Body).Decode(&req)

	resp, err := g.orderClient.CreateOrder(context.Background(), &req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp)
}
