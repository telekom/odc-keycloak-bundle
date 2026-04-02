# Database

This document explains the decision to use PostgreSQL managed by the CloudNativePG operator as the database backend for Keycloak.

## Decision

**PostgreSQL** is the database backend for all Keycloak instances, managed by the **CloudNativePG** (CNPG) operator. This is consistent with the database choice across the wider project family, where PostgreSQL with CNPG is the standard persistence layer.

## Why PostgreSQL

Keycloak requires a persistent database for realms, users, clients, sessions, and audit data. Several options were evaluated:

| Option | Verdict | Reason |
|--------|---------|--------|
| **PostgreSQL** | Selected | Production-proven with Keycloak, strong consistency, ACID compliance, good IAM workload performance |
| MySQL / MariaDB | Rejected | Less ecosystem alignment, no advantage over PostgreSQL for this workload |
| Embedded H2 | Development only | Not suitable for production; no replication, no persistence guarantees |

PostgreSQL was the clear choice given the customer's existing expertise, its production track record with Keycloak, and the project-wide standardization on PostgreSQL for all OCM components.

## Why CloudNativePG

The operator manages the PostgreSQL lifecycle (provisioning, failover, backups, updates) declaratively. Four operators were evaluated:

| Operator | Verdict | Key Characteristics |
|----------|---------|---------------------|
| **CloudNativePG** | Selected | CNCF Sandbox, lightweight single binary, native Kubernetes integration, no Patroni dependency |
| Zalando Postgres Operator | Rejected | More complex, Patroni-based, better suited for large-scale multi-tenant setups |
| CrunchyData PGO | Rejected | Enterprise-focused, heavier footprint |
| Percona Operator | Rejected | Newer project, less community adoption |

CloudNativePG fits the project requirements well:

- **Lightweight** -- single-binary operator, minimal resource overhead
- **Declarative** -- PostgreSQL clusters defined as Kubernetes CRs
- **Namespace-compatible** -- per-namespace clusters align naturally with the multi-instance isolation model (see [ARCHITECTURE.md](ARCHITECTURE.md))
- **Air-gap ready** -- no external dependencies at runtime
- **Backup/Restore** -- integrated Barman support for point-in-time recovery

## Deployment Model

The CNPG operator is installed once at the cluster level. Each Keycloak instance gets its own PostgreSQL cluster in its namespace.

### Operator Installation (cluster-wide, once)

```bash
kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.28/releases/cnpg-1.28.1.yaml
```

### Per-Instance PostgreSQL Cluster

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: keycloak-db
spec:
  instances: 1    # 3 for high availability
  storage:
    size: 5Gi
  bootstrap:
    initdb:
      database: keycloak
      owner: keycloak
```

### Connection Details

| Parameter | Value |
|-----------|-------|
| Host | `keycloak-db-rw.<namespace>.svc` |
| Port | `5432` |
| Database | `keycloak` |
| Credentials | Auto-generated in Secret `keycloak-db-app` |

Keycloak connects to the `-rw` (read-write) service endpoint, which always points to the current primary.

## Trade-offs

**Benefits:**
- Reliable, well-understood technology with broad tooling support
- Operator handles failover, rolling updates, and backups automatically
- Per-namespace clusters enforce data isolation between instances
- Consistent with the database strategy across all project OCM components

**Costs:**
- The CNPG operator requires cluster-wide installation (CRDs and controller)
- Additional CRDs to manage alongside the Keycloak CRDs
- Team needs familiarity with CNPG-specific configuration and troubleshooting

## Decision Record

*Decision: Use PostgreSQL managed by CloudNativePG as the database backend for all Keycloak instances.*

*Date: 2026-01-31. Decision makers: Project Team, Customer.*

PostgreSQL is the project-wide standard for all OCM components. The customer has existing PostgreSQL expertise and requires a production-proven, ACID-compliant database that works reliably in air-gapped environments. MySQL/MariaDB offered no advantage for this workload and would have introduced a second database technology. Embedded H2 is suitable only for development.

Among PostgreSQL operators, CloudNativePG was selected for its lightweight single-binary architecture and native Kubernetes integration without the Patroni dependency that Zalando's operator requires. CrunchyData PGO was considered too enterprise-heavy, and Percona's operator had insufficient community adoption at the time of evaluation.

The main trade-off is that the CNPG operator must be installed cluster-wide (CRDs + controller), adding a cluster-level dependency. This was accepted because the operator is shared across all instances and the installation is a one-time operation.

## Related Documents

| Topic | Document |
|-------|----------|
| Multi-instance namespace isolation | [ARCHITECTURE.md](ARCHITECTURE.md) |
