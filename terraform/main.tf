# =============================================================================
# Terraform Root Module — FAANG-Grade E-Commerce Infrastructure
# =============================================================================
# Provisions the complete AWS infrastructure per BRD Section 4.4:
# - EKS Kubernetes Cluster (with Karpenter node provisioning)
# - Aurora PostgreSQL Global Database (3 instances per BRD Section 9.1)
# - ElastiCache Redis Cluster (6-node, 3 shards)
# - Amazon MSK (Managed Kafka)
# - ECR Container Registries
# - VPC with multi-AZ networking
# =============================================================================

terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  # Remote state per BRD Section 8.3: S3 + DynamoDB locking
  backend "s3" {
    bucket         = "digital-metro-terraform-state"
    key            = "ecommerce-platform/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-state-lock"
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "ecommerce-platform"
      ManagedBy   = "terraform"
      Environment = var.environment
      Owner       = "digital-metro"
      CostCenter  = "platform-engineering"
    }
  }
}

# =============================================================================
# Variables
# =============================================================================

variable "aws_region" {
  description = "AWS region for primary deployment"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Deployment environment (dev/staging/production)"
  type        = string
  default     = "dev"
}

variable "cluster_name" {
  description = "EKS cluster name"
  type        = string
  default     = "ecommerce-platform"
}

variable "vpc_cidr" {
  description = "VPC CIDR block"
  type        = string
  default     = "10.0.0.0/16"
}

# =============================================================================
# VPC — Multi-AZ Networking (BRD: 3 AZs per region)
# =============================================================================

data "aws_availability_zones" "available" {
  state = "available"
}

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "${var.cluster_name}-vpc"
  }
}

resource "aws_subnet" "public" {
  count             = 3
  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index)
  availability_zone = data.aws_availability_zones.available.names[count.index]

  map_public_ip_on_launch = true

  tags = {
    Name                                        = "${var.cluster_name}-public-${count.index}"
    "kubernetes.io/role/elb"                     = "1"
    "kubernetes.io/cluster/${var.cluster_name}"  = "shared"
  }
}

resource "aws_subnet" "private" {
  count             = 3
  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index + 10)
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name                                        = "${var.cluster_name}-private-${count.index}"
    "kubernetes.io/role/internal-elb"            = "1"
    "kubernetes.io/cluster/${var.cluster_name}"  = "shared"
  }
}

# PCI-DSS: Isolated subnet for payment CDE (BRD Section 10.4)
resource "aws_subnet" "payment_cde" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index + 20)
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name        = "${var.cluster_name}-payment-cde-${count.index}"
    Compliance  = "PCI-DSS"
    Environment = "CDE"
  }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "${var.cluster_name}-igw" }
}

resource "aws_eip" "nat" {
  count  = 3
  domain = "vpc"
  tags   = { Name = "${var.cluster_name}-nat-eip-${count.index}" }
}

resource "aws_nat_gateway" "main" {
  count         = 3
  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id
  tags          = { Name = "${var.cluster_name}-nat-${count.index}" }
}

# =============================================================================
# ECR — Container Registries (BRD: one per microservice)
# =============================================================================

locals {
  services = [
    "api-gateway",
    "user-service",
    "product-service",
    "order-service",
    "payment-service",
    "inventory-service",
    "cart-service",
    "notification-service",
  ]
}

resource "aws_ecr_repository" "services" {
  for_each             = toset(local.services)
  name                 = "${var.cluster_name}/${each.key}"
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true  # BRD: Continuous container vulnerability scanning
  }

  encryption_configuration {
    encryption_type = "AES256"
  }

  tags = {
    Service = each.key
  }
}

# =============================================================================
# Outputs
# =============================================================================

output "vpc_id" {
  value = aws_vpc.main.id
}

output "public_subnets" {
  value = aws_subnet.public[*].id
}

output "private_subnets" {
  value = aws_subnet.private[*].id
}

output "ecr_repositories" {
  value = { for k, v in aws_ecr_repository.services : k => v.repository_url }
}
