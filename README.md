[English](./README.md) | [简体中文](./README_zh.md)

# KubeMin-Cli

**A Developer-Friendly Application Platform for Kubernetes.**

KubeMin-Cli is a cloud-native application management platform designed to dramatically simplify the deployment and orchestration of services on Kubernetes. It provides a high-level, developer-centric abstraction layer that bridges the gap between simple application needs and the underlying complexity of Kubernetes.

---

## Vision

Our goal is to provide a single, cohesive user experience for defining, deploying, and managing applications on Kubernetes. We believe developers shouldn't have to be Kubernetes experts to build and run their services. KubeMin-Cli abstracts away the boilerplate and allows teams to focus on what truly matters: their application logic.

## Core Concepts

KubeMin-Cli is built on a set of powerful, yet intuitive, concepts:

*   **Component Model:** Inspired by the [Open Application Model (OAM)](https://oam.dev/), your application is composed of one or more **Components**. A component is a runnable unit of your application, like a web API, a worker process, or a database.
*   **Traits:** Components are augmented with **Traits**, which attach operational features. Need persistent storage, a sidecar container, or specific environment variables? Simply apply a Trait. This keeps your core application definition clean and focused.
*   **Workflow:** A **Workflow** defines the relationship between your components and the process of deploying them. It describes dependencies (e.g., "deploy the database before the API") and orchestration logic.

## Architecture: A Lightweight Workflow Engine

The heart of KubeMin-Cli is its native, lightweight workflow engine. This engine is implemented as a custom Kubernetes controller that directly interprets the `KubeMinWorkflow` definition and translates it into fundamental Kubernetes resources like Deployments, Jobs, and Services.

This approach is optimized for simplicity, efficiency, and low resource consumption, making it ideal for the most common application deployment scenarios and resource-constrained environments like edge computing.

## Features

*   **Unified Application Definition:** Define your entire application, its operational characteristics, and its deployment process in one simple, declarative model.
*   **Extensible Trait System:** Easily add capabilities like storage, configuration, networking, and more to any component.
*   **Lightweight & Efficient:** Runs with minimal overhead, making it suitable for any Kubernetes cluster from a local developer machine to the edge.


## Roadmap

Our vision extends to the most demanding cloud-native workloads. Future versions plan to introduce:

*   **A Hybrid Workflow Engine:** To provide first-class support for complex AI/ML pipelines, we plan to introduce an optional, pluggable execution engine based on industry-standard tools like **Argo Workflows**. This will enable advanced features like complex DAGs, artifact passing, and event-driven execution.
*   **Advanced AI/ML Workload Support:** Simplify the entire lifecycle of AI/ML applications, from training to serving, by integrating with frameworks like KubeFlow and KServe.

## Getting Started

*(Coming soon...)*

## Contributing

We welcome contributions! Please see our contributing guidelines for more details.

*(Coming soon...)*