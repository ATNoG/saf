# Serverless Application Firewall (SAF)

A security extension for Knative Serving that provides firewall capabilities for serverless applications. SAF integrates with Knative's `queue-proxy` to filter and validate HTTP requests and responses based on configurable rules.

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Architecture](#architecture)
- [Installation](#installation)
- [Firewall Language](#firewall-language)
- [CNCF Serverless Workflow Integration](#cncf-serverless-workflow-integration)
- [Usage](#usage)
- [Performance Evaluation](#performance-evaluation)
- [Configuration Reference](#configuration-reference)
- [Important Notes](#important-notes)
- [License](#license)
- [Support](#support)

## Overview

The Serverless Application Firewall (SAF) is designed to enhance the security of serverless applications running on Knative. It operates as an extension to Knative's queue-proxy, allowing for fine-grained control over HTTP traffic through configurable rules.

## Key Features

- **Request and Response Filtering**: Validate both incoming requests and outgoing responses
- **Flexible Rule System**: Use jq expressions for powerful pattern matching and validation
- **Knative Integration**: Seamless integration with Knative Serving through annotations
- **Preliminary Testing**: Performance evaluation framework included

## Architecture

The SAF consists of several key components:

### 1. Queue Proxy Firewall

The core firewall implementation that integrates with Knative's queue-proxy:

- **Location**: [`queue-proxy-firewall/pkg/firewall/`](queue-proxy-firewall/pkg/firewall/)
- **Language**: Go
- **Key Features**:
  - Implements Knative's `RoundTripPlug` interface
  - Request and response validation
  - Rule evaluation using jq expressions

### 2. Firewall Language

A dedicated language for defining firewall rules:

- **Location**: [`rules-reference/`](rules-reference/)
- **Purpose**: Define security rules in a structured format
- **Components**:
  - Schema definition and validation
  - Example rules and patterns
  - Rule extraction tools

### 3. Evaluation Framework

Performance testing and benchmarking tools:

- **Location**: [`evaluation/`](evaluation/)
- **Purpose**: Measure firewall performance impact
- **Components**:
  - Invocation and teardown performance tests
  - Latency performance tests
  - Data analysis and visualization tools

### 4. Sample Applications

Example applications demonstrating firewall usage:

- **Location**: [`apps/`](apps/)
- **Examples**:
  - Bank application with multiple services
  - Simple sample application for testing

## Installation

### Prerequisites

- Kubernetes cluster with Knative Serving installed
- `ko` for container image building
- Go 1.16+ for development

### Building the Firewall

```bash
cd queue-proxy-firewall
./build.sh
```

This builds and pushes the queue-proxy container image with firewall capabilities to your container registry.

### Use the SAF Firewall

```bash
cd queue-proxy-firewall
./patch.sh
```

This will set the `queue-proxy` image as the SAF firewall.

### Deploying Applications to Kubernetes

```bash
# Apply a sample application
kubectl apply -f apps/sample-app/kubernetes.yaml

# Or deploy the bank application example (you will also need SonataFlow installed)
kubectl apply -f apps/bank/kubernetes.yaml
```

## Firewall Language

SAF includes a dedicated language for defining firewall rules with a formal schema and validation system.

### Language Overview

The firewall language is defined in [`rules-reference/firewall-schema.yaml`](rules-reference/firewall-schema.yaml) and provides:

- **Structured Rule Definition**: YAML-based format for firewall rules
- **Schema Validation**: JSON Schema validation for rule syntax
- **jq Expression Support**: Powerful pattern matching using jq expressions
- **Context Variables**: Access to request and response context

### Schema Definition

The schema defines the structure for firewall rules:

```yaml
firewall:
  request:
    default-action: accept  # accept, reject, or drop
    rules:
      - action: reject       # accept, reject, drop, or log
        expression: "jq expression"
        # Optional schema validation
        schema:
          type: json|yaml
          path: "path/to/schema"
  response:
    default-action: accept
    rules:
      - action: reject
        expression: "jq expression"
```

## CNCF Serverless Workflow Integration

SAF provides tools for integrating firewall rules with CNCF Serverless Workflows (for example for usage in SonataFlow).

### Workflow Extractor

The [`workflow-extractor.py`](rules-reference/workflow-extractor.py) script:

1. **Extracts rules** from workflow function metadata
2. **Validates rules** against the firewall schema
3. **Injects rules** into Kubernetes configuration files

### Usage

```bash
cd rules-reference
python workflow-extractor.py
```

This processes workflows like [`apps/bank/workflow/src/main/resources/workflow.sw.yaml`](apps/bank/workflow/src/main/resources/workflow.sw.yaml) and automatically adds firewall rules to the corresponding Knative Services in the Kubernetes configuration.

### Workflow Example

In the bank workflow example, the login function includes comprehensive firewall rules:

```yaml
functions:
  - name: login
    type: custom
    operation: knative:services.v1.serving.knative.dev/login?method=POST
    metadata:
      firewall:
        request:
          default-action: accept
          rules:
            - action: drop
              expression: |
                .REQUEST.METHOD | ascii_upcase | . != "POST"
            # Additional rules for input validation
        response:
          default-action: accept
          rules:
            - action: drop
              expression: |
                .RESPONSE.BODY.error? | type == "string"
```

### Generated Kubernetes Configuration

The extractor generates Kubernetes configurations with the rules embedded:

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: login
spec:
  template:
    metadata:
      annotations:
        qpoption.knative.dev/firewall-activate: "enable"
        qpoption.knative.dev/firewall-config-rules-json: '{"request": {"default-action": "accept", "rules": [...]}}'
```

## Usage

### Basic Configuration

The firewall is configured through Kubernetes annotations on Knative Services:

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: my-service
spec:
  template:
    metadata:
      annotations:
        qpoption.knative.dev/firewall-activate: "enable"
        qpoption.knative.dev/firewall-config-rules: |
          request:
            default-action: accept
            rules:
              - action: reject
                expression: |
                  (.REQUEST.BODY.username | test("admin"))
```

### Rule Configuration

Rules are defined using a YAML structure with jq expressions:

```yaml
request:
  default-action: accept  # or reject
  rules:
    - action: reject       # accept, reject, or drop
      expression: |
        (.REQUEST.HEADERS.User-Agent | test("malicious-bot"))
    - action: accept
      expression: |
        (.REQUEST.BODY.api_key == "valid-key")

response:
  default-action: accept
  rules:
    - action: reject
      expression: |
        (.RESPONSE.BODY.error != null)
```

### Available Context Variables

- **REQUEST**: Contains request information
  - `HEADERS`: HTTP headers
  - `BODY`: Parsed request body (JSON)
  - `METHOD`: HTTP method
  - `PATH`: Request path
  - `QUERY`: Query parameters

- **RESPONSE**: Contains response information
  - `HEADERS`: HTTP response headers
  - `BODY`: Parsed response body (JSON)
  - `STATUS`: HTTP status code

## Performance Evaluation

The evaluation framework provides comprehensive performance testing:

### Running Tests

```bash
# Invocation performance tests (deployment/teardown times)
cd evaluation/invocation
./test.sh

# Latency performance tests (request/response processing)
cd evaluation/latency
./test.sh
```

### Generating Graphs

```bash
# Invocation/teardown
cd evaluation/invocation
python graphs.py

# Latency
cd evaluation/latency
python graphs.py
```

This generates PDF graphs showing performance impact with different rule counts.

### Test Results Structure

Results are stored in:
- [`evaluation/invocation/results/`](evaluation/invocation/results/)
- [`evaluation/latency/results/`](evaluation/latency/results/)

Each test run creates a timestamped directory with:
- Request traces
- Pod logs
- Performance metrics

### Obtained Results

Graphs are stored in:
- Deployment: [evaluation/invocation/deployment-automatic-gc.pdf](evaluation/invocation/deployment-automatic-gc.pdf)
- Teardown: [evaluation/invocation/teardown-automatic-gc.pdf](evaluation/invocation/teardown-automatic-gc.pdf)
- Deployment with manual GC: [evaluation/invocation/deployment-manual-gc.pdf](evaluation/invocation/deployment-manual-gc.pdf)
- Teardown with manual GC: [evaluation/invocation/teardown-manual-gc.pdf](evaluation/invocation/teardown-manual-gc.pdf)
- Latency: [evaluation/latency/sample-app.pdf](evaluation/latency/sample-app.pdf)

## Configuration Reference

### Annotations

| Annotation | Description | Values |
|-----------|-------------|--------|
| `qpoption.knative.dev/firewall-activate` | Enable/disable firewall | `enable`, `disable` |
| `qpoption.knative.dev/firewall-config-rules` | Firewall rules configuration | YAML string |

### Rule Actions

| Action | Description |
|--------|-------------|
| `accept` | Allow the request/response |
| `reject` | Block with error response |
| `drop` | Silent block (currently same as reject) |

## Important Notes

**Development Warning**: The firewall logs "Never use in production" during initialization

## License

This project is licensed under the GPL-3.0 License - see the [LICENSE](LICENSE) file for details.

## Support

For issues and questions, please open an issue in the GitHub repository or send an [e-mail](mailto:escaleira@av.it.pt).
