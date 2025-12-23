[English](./README.md) | [简体中文](./README_zh.md)

# KubeMin-Cli

**A Developer-Friendly Kubernetes Application Platform**

KubeMin-Cli is a cloud-native PaaS platform that uses lightweight workflows to describe applications. Its structural model is based on the **OAM (Open Application Model)**, with a natively designed workflow engine that drives the complete application lifecycle: initializing resources, creating/updating workloads, publishing events, monitoring status, and more.

## Core Capabilities

### 1. Traits: Composable Capability Atoms

The Traits structure abstracts Kubernetes concepts like Pods, Services, and Volumes. It defines storage, container sidecars, environment variables, initialization tasks, secrets, and more as Traits, allowing components to be built through composition rather than complex YAML files. The "template import" feature enables rapid creation of multiple applications with the same form (e.g., multiple MySQL instances), ensuring generated resources are available, conflict-free, and traceable.

### 2. Workflow: Task-Driven Application Lifecycle

Operations like "deploy," "update," and "scale" are abstracted into workflow instances that support parallel tasks, state consistency, component dependencies, and persistent execution records. The infrastructure layer uses List-Watch patterns to maintain the entire application state lifecycle and real-time synchronization between application components.

## Architecture Overview

### Workflow Engine

The KubeMin-Cli workflow engine is the core component of the application delivery system, responsible for translating declared application configurations into actual Kubernetes cluster resources. It acts as an "orchestrator," coordinating the creation, update, and deletion of multiple components to ensure correct application deployment.

#### Execution Modes

1. **Local Mode**: Uses NoopQueue, directly scans the database to execute tasks, suitable for single-instance deployment and development testing.
2. **Distributed Mode**: Uses Redis Streams, Kafka, etc.; supports task distribution and failure recovery, suitable for multi-instance deployment and production environments.

#### Key Features

- **Resource Dependency Management**: Ensures ConfigMaps, Secrets, PVCs, and other dependent resources are created before Deployments and StatefulSets
- **Execution Order Control**: Supports serial and parallel execution modes for different scenarios
- **Status Tracking**: Complete recording of each task and Job execution status for troubleshooting
- **Fault Recovery**: Supports task retry, cancellation, and resource cleanup to ensure system consistency
- **Distributed Scaling**: Supports multi-instance deployment with task distribution via Redis Streams/Kafka

#### Workflow Definition Example

```json
{
  "workflow": [
    {
      "name": "config-step",
      "mode": "StepByStep",
      "components": ["config", "secret"]
    },
    {
      "name": "database",
      "mode": "DAG",
      "components": ["mysql", "redis"]
    },
    {
      "name": "services",
      "mode": "StepByStep",
      "components": ["backend", "frontend"]
    }
  ]
}
```

### OAM Traits

KubeMin-Cli provides a comprehensive set of Traits for augmenting components:

#### Available Traits

| Trait | Description | K8s Resources |
|-------|-------------|---------------|
| Storage | Storage mounting | PVC, EmptyDir, ConfigMap, Secret Volume |
| Init | Initialization containers | InitContainer |
| Sidecar | Sidecar containers | Container |
| Envs | Individual environment variables | EnvVar |
| EnvFrom | Batch environment variable import | EnvFromSource |
| Probes | Health check probes | LivenessProbe, ReadinessProbe, StartupProbe |
| Resources | Compute resource limits | ResourceRequirements |
| Ingress | Ingress traffic routing | Ingress |
| RBAC | Permission access control | ServiceAccount, Role, RoleBinding, ClusterRole, ClusterRoleBinding |

#### Trait Processing Order

1. Storage
2. EnvFrom
3. Envs
4. Resources
5. Probes
6. RBAC
7. Init
8. Sidecar
9. Ingress

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.20+)
- MySQL database
- Redis (for distributed mode)

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/KubeMin-Cli.git
cd KubeMin-Cli

# Build the binary
go build -o kubemin-cli cmd/main.go

# Run the server
./kubemin-cli
```

### Deploy Your First Application

```bash
# Create an application
curl -X POST http://localhost:8080/applications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "demo-app",
    "namespace": "default",
    "component": [
      {
        "name": "web",
        "type": "webservice",
        "image": "nginx:latest",
        "replicas": 2,
        "properties": {
          "ports": [{"port": 80, "expose": true}]
        }
      }
    ],
    "workflow": [
      {
        "name": "deploy",
        "mode": "StepByStep",
        "components": ["web"]
      }
    ]
  }'
```

## Configuration

### Environment Variables

- `MYSQL_DSN`: MySQL connection string
- `REDIS_ADDR`: Redis server address (for distributed mode)
- `KUBECONFIG`: Path to kubeconfig file

### Workflow Engine Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `--workflow-sequential-max-concurrency` | 1 | Max concurrency within serial steps |
| `--workflow-max-concurrent` | 10 | Max concurrent workflows |
| `--msg-type` | redis | Message queue type (noop/redis/kafka) |

## Development

### Build

```bash
# Build for current OS
go build -o kubemin-cli cmd/main.go

# Build for Linux
make build-linux

# Build for macOS
make build-darwin

# Build for Windows
make build-windows
```

### Testing

```bash
# Run all tests with race detection and coverage
go test ./... -race -cover

# Run specific package tests
go test ./pkg/apiserver/workflow/... -v
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run static analysis
go vet ./...

# Tidy dependencies
go mod tidy
```

## Architecture Details

### Clean Architecture Structure

```
pkg/apiserver/
├── domain/           # Business logic and domain models
│   ├── model/        # Domain entities
│   ├── service/      # Business logic services
│   └── repository/   # Data access interfaces
├── infrastructure/   # External integrations
│   ├── persistence/  # Database layer (GORM + MySQL)
│   ├── messaging/    # Queue implementations
│   ├── kubernetes/   # K8s client and utilities
│   └── tracing/      # OpenTelemetry integration
├── interfaces/api/   # REST API layer
│   ├── handlers/     # HTTP request handlers
│   └── middleware/   # Gin middleware
├── workflow/         # Workflow execution engine
│   ├── dispatcher/   # Job distribution logic
│   ├── worker/       # Job execution workers
│   └── traits/       # Component trait processors
└── utils/            # Shared utilities
```

### Key Patterns

1. **Dependency Injection**: Custom IoC container manages service lifecycle
2. **Queue Abstraction**: Unified interface supporting Redis Streams and local queues
3. **Trait System**: Extensible component augmentation through trait processors
4. **Leader Election**: Kubernetes Lease-based leader election for distributed mode

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
