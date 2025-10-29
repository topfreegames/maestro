# System Patterns - Maestro Architecture

## Overview

Maestro Next is a modular system composed of different components that can be executed independently. All modules share the same codebase but are launched with specific command-line arguments: `go run main.go start [MODULE_NAME]`

## Core Architecture

```mermaid
graph TB
    subgraph "Maestro System"
        MA[Management API]
        RA[Rooms API]
        OEW[Operation Execution Worker]
        RWW[Runtime Watcher Worker]
        MRW[Metrics Reporter Worker]
    end

    subgraph "Shared Storages"
        REDIS[Redis Storage]
        PG[PostgreSQL]
    end

    subgraph "Runtime Infrastructure"
        K8S[Kubernetes Runtime]
    end

    subgraph "External Actors"
        OPERATORS[Operators]
        GR[Game Rooms]
    end

    OPERATORS --> MA
    MA --> REDIS
    MA --> PG
    RA --> GR
    RA --> REDIS
    RA --> PG
    OEW <--> |Deploy/Scale ↔ Pod Events| K8S
    OEW <--> REDIS
    OEW --> PG
    RWW <--> |Watch ↔ Status Changes| K8S
    RWW --> REDIS
    RWW --> PG
    MRW --> |Metrics Collection| K8S
    MRW --> REDIS
    MRW --> PG
```

## Module Components

### 1. Management API
**Purpose**: Primary interface for user requests and system management

**Key Responsibilities**:
- Accepts gRPC and HTTP requests
- Manages schedulers and operations
- Provides system-wide coordination

**Services**:
- **Schedulers Service**: Create, fetch, update schedulers
- **Operations Service**: Track and manage operation status

**Data Dependencies**:
- Redis: Operations and game rooms data
- PostgreSQL: Scheduler persistence

```mermaid
graph LR
    subgraph "Management API"
        SS[Schedulers Service]
        OS[Operations Service]
    end

    subgraph "Storage"
        R[Redis]
        P[PostgreSQL]
    end

    subgraph "External Actors"
        O[Game Operators, Frontend, Maestro CLI]
    end

    O --> SS
    O --> OS
    SS --> P
    OS --> R
    OS --> P
```

### 2. Rooms API
**Purpose**: Game room status synchronization and event forwarding

**Key Responsibilities**:
- Sync game room status with Maestro
- Forward events to configured destinations
- Maintain real-time room state

**Critical Notes**:
- **REQUIRED** for game rooms to function properly
- Must receive constant status updates from managed rooms
- Handles event forwarding for schedulers with forwarders configured

**Integration**:
- Uses [Maestro client](https://github.com/topfreegames/maestro-client) for easier integration
- Events protocol: [Proto file reference](https://github.com/topfreegames/protos/blob/master/maestro/grpc/protobuf/events.proto)

```mermaid
graph LR
    subgraph "Rooms API"
        RS[Rooms Service]
        EF[Event Forwarder]
    end

    subgraph "Game Rooms"
        GR1[Game Room 1]
        GR2[Game Room 2]
        GRN[Game Room N]
    end

    subgraph "External Matchmaking"
        F1[Forwarder 1]
        F2[Forwarder 2]
    end

    subgraph "Shared Storages"
        SQL[PostgreSQL]
        RED[Redis]
    end

    RS --> SQL
    RS --> RED
    GR1 --> RS
    GR2 --> RS
    GRN --> RS
    RS --> EF
    EF --> F1
    EF --> F2
```

### 3. Operation Execution Worker
**Purpose**: Executes operations for active schedulers

**Key Concept**: Each worker manages operations for **one and only one Scheduler**

**Workflow**:
1. Monitors active schedulers
2. Creates execution threads per scheduler
3. Processes operations from scheduler queues
4. Tracks operation events and status

**Benefits**:
- Enables operation tracking
- Provides healthy scheduler changes
- Maintains execution isolation

```mermaid
graph TB
    subgraph "Operation Execution Worker"
        WM[Workers Manager]
    end

    subgraph "Schedulers"
        S1[Scheduler 1]
        S2[Scheduler 2]
        S3[Scheduler 3]
    end

    subgraph "Execution Threads"
        ET1[Scheduler1 Thread]
        ET2[Scheduler2 Thread]
        ET3[Scheduler3 Thread]
    end

    subgraph "Shared Storages"
        SQL[PostgreSQL]
        RED[Redis]
    end

    subgraph "Runtime Infrastructure"
        K8S[Kubernetes API]
    end

    WM --> | Manage Namespaces, Pods, PDBs | K8S

    ET1 --> | Fetch Scheduler Data | SQL
    ET1 --> | Read and Create Operations | RED

    ET2 --> SQL
    ET2 --> RED

    ET3 --> SQL
    ET3 --> RED


    WM --> S1
    WM --> S2
    WM --> S3

    S1 --> ET1
    S2 --> ET2
    S3 --> ET3
```

### 4. Runtime Watcher Worker
**Purpose**: Monitors runtime events and reflects changes in Maestro

**Key Concept**: Each worker manages **one and only one Scheduler**

**Monitoring Capabilities**:
- Game room creation events
- Game room deletion events
- Game room update events
- Disruption mitigation based on occupied rooms

**Runtime Integration**: Currently supports Kubernetes only

```mermaid
graph TB
    subgraph "Runtime Watcher Worker"
        RWW[Runtime Watcher]
    end

    subgraph "Schedulers Threads"
        S1[Scheduler 1]
        S2[Scheduler 2]
        SN[Scheduler N]
    end

    subgraph "Pods Informer"
        PI1[Pods Informer 1]
        PI2[Pods Informer 2]
        PIN[Pods Informer N]
    end

    subgraph "Shared Storages"
        SQL[PostgreSQL]
        RED[Redis]
    end

    subgraph "Runtime Infrastructure"
        K8S[Kubernetes API]
    end

    RWW <--> | List / Watch Pods | K8S
    RWW <--> | Update PDB | K8S

    RWW --> S1
    RWW --> S2
    RWW --> SN

    PI1 --> S1
    PI2 --> S2
    PIN --> SN

    S1 --> RED
    S1 --> SQL

    S2 --> RED
    S2 --> SQL

    SN --> RED
    SN --> SQL
```

### 5. Metrics Reporter Worker
**Purpose**: Reports runtime metrics and game room status

**Key Concept**: Each worker manages **one and only one Scheduler**

**Metrics Collected**:
- **Runtime Status**: `ready`, `pending`, `error`, `unknown`, `terminating`
- **Storage Status**: `ready`, `pending`, `error`, `occupied`, `terminating`, `unready`

**Important Note**: This module is **optional** - not required for core functionality

```mermaid
graph TB
    subgraph "Metrics Reporter Worker"
        MRW[Metrics Reporter]
    end

    subgraph "Schedulers Threads"
        S1[Scheduler 1]
        S2[Scheduler 2]
        SN[Scheduler N]
    end

    subgraph "Shared Storages"
        SQL[PostgreSQL]
        RED[Redis]
    end

    subgraph "Metrics"
        PR[Prometheus]
    end

    MRW --> S1
    MRW --> S2
    MRW --> SN

    MRW --> | Fetch Scheduler Info | SQL
    MRW --> | Cache Scheduler Info | RED
    MRW --> | Read Game Room Stats | RED

    S1 --> PR
    S2 --> PR
    SN --> PR
```

## System Constraints

### Runtime Support
- **Current**: Kubernetes only
- **Future**: Extensible to other runtime systems

### Worker Isolation
- Each worker operates on **one scheduler only**
- Prevents parallel execution conflicts
- Ensures clean operation tracking

### Data Flow Patterns
- **Redis**: High-frequency, temporary data (operations, room status)
- **PostgreSQL**: Persistent, structured data (schedulers)
- **Kubernetes**: Runtime state and orchestration

## Key Design Principles

1. **Modularity**: Each component has distinct responsibilities
2. **Isolation**: Workers operate independently per scheduler
3. **Observability**: Comprehensive operation tracking and metrics
4. **Scalability**: Horizontal scaling through worker instances
5. **Reliability**: Event-driven architecture with state persistence
