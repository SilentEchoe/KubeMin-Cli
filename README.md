[English](./README.md) | [简体中文](./README_zh.md)

# KubeMin-Cli

**A Unified Application Platform for Kubernetes, from Edge to AI.**

KubeMin-Cli is a next-generation cloud-native application management platform designed to dramatically simplify the deployment and orchestration of services on Kubernetes. It provides a high-level, developer-centric abstraction layer that bridges the gap between simple application needs and the underlying complexity of Kubernetes, empowering developers to deploy everything from lightweight microservices on edge devices to complex AI/ML pipelines in the data center.

---

## Vision

Our goal is to provide a single, cohesive user experience for defining, deploying, and managing applications on Kubernetes. We believe developers shouldn't have to be Kubernetes experts to build and run their services. KubeMin-Cli abstracts away the boilerplate and allows teams to focus on what truly matters: their application logic.

## Core Concepts

KubeMin-Cli is built on a set of powerful, yet intuitive, concepts:

*   **Component Model:** Inspired by the [Open Application Model (OAM)](https://oam.dev/), your application is composed of one or more **Components**. A component is a runnable unit of your application, like a web API, a worker process, or a database.
*   **Traits:** Components are augmented with **Traits**, which attach operational features. Need persistent storage, a sidecar container, or specific environment variables? Simply apply a Trait. This keeps your core application definition clean and focused.
*   **Workflow:** A **Workflow** defines the relationship between your components and the process of deploying them. It describes dependencies (e.g., "deploy the database before the API") and orchestration logic, from simple rollouts to complex, multi-stage pipelines.

## Architecture: The Hybrid Workflow Engine

The heart of KubeMin-Cli is its unique **Hybrid Workflow Engine**. This architecture provides the ultimate flexibility by decoupling the user-facing workflow definition from the underlying execution engine, allowing the platform to choose the right tool for the right job.

### 1. Native Engine (Lightweight & Edge-Optimized)

*   **How it works:** A custom, lightweight Kubernetes controller that directly interprets the `KubeMinWorkflow` and orchestrates basic Kubernetes resources (Deployments, Jobs, Services).
*   **Best for:**
    *   Standard microservice and web application deployments.
    *   Resource-constrained environments like **edge computing** (K3s, MicroK8s).
    *   Scenarios where minimal overhead and zero extra dependencies are critical.
*   **Advantages:** Extremely low resource footprint, simple to monitor, and no third-party dependencies.

### 2. Argo-based Engine (Powerful & AI-Ready)

*   **How it works:** KubeMin-Cli acts as a control plane that translates the `KubeMinWorkflow` into a native **Argo Workflows** CRD. It then delegates the execution to a full-featured Argo Workflows engine.
*   **Best for:**
    *   Complex, multi-stage **AI/ML pipelines** (e.g., data prep -> training -> evaluation -> serving).
    *   Workflows requiring advanced features like Directed Acyclic Graph (DAG) logic, artifact passing (e.g., sharing a trained model between steps), and event-driven triggers.
    *   Integrating with the broader MLOps ecosystem (KubeFlow, KServe).
*   **Advantages:** Leverages a battle-tested, industry-standard engine to handle maximum complexity without reinventing the wheel.

This hybrid approach ensures that KubeMin-Cli is both simple enough for a single container deployment and powerful enough for a distributed model training job.

## Features

*   **Unified Application Definition:** Define your entire application, its operational characteristics, and its deployment process in one simple, declarative model.
*   **Extensible Trait System:** Easily add capabilities like storage, configuration, networking, and more to any component.
*   **Hybrid Execution:** Automatically leverage the best workflow engine for your needs, from lightweight to powerful.
*   **Developer-Friendly:** Abstract away Kubernetes YAML complexity with a clean, intuitive interface.
*   **AI/ML Ready:** Built from the ground up to support the entire lifecycle of modern AI applications.

## Getting Started

*(Coming soon...)*

## Contributing

We welcome contributions! Please see our contributing guidelines for more details.

*(Coming soon...)*
