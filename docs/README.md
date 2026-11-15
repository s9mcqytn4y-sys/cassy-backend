# Cassy Backend Docs

Konvensi: folder dan file memakai **PascalCase** (`Architecture/Roadmap.md`). `README.md`, `AGENTS.md`, `CLAUDE.md`, dan `GEMINI.md` memakai konvensi standar. Dokumen historis berada di `Archive/`.

| Folder | Isi | Mulai dari |
|---|---|---|
| `ProjectManagement/` | Source of Truth backlog, sprint, status rilis lintas repositori | [README.md](ProjectManagement/README.md), [BacklogAndDebt.md](ProjectManagement/BacklogAndDebt.md) |
| `Product/` | Merek, kebutuhan per peran, modul, PRD, re-alignment, analisis kompetitor | [ReAlignment2026-11.md](Product/ReAlignment2026-11.md), [BrandAndProductKnowledge.md](Product/BrandAndProductKnowledge.md) |
| `Architecture/` | ADR (ADR-002 s.d. ADR-010), keputusan owner, roadmap, kontrak API/DB, risiko | [Roadmap.md](Architecture/Roadmap.md), [Adr009MultiTenantSaasGovernanceAndByok.md](Architecture/Adr009MultiTenantSaasGovernanceAndByok.md) |
| `Api/` | Kontrak OpenAPI 3.1 (sumber kebenaran endpoint) | [OpenApi.yaml](Api/OpenApi.yaml), [FrontendIntegrationContract.md](Architecture/FrontendIntegrationContract.md) |
| `Engineering/` | Backlog audit, workflow Git dan rilis, gaya Go, audit dependensi | [GitWorkflowAndReleases.md](Engineering/GitWorkflowAndReleases.md), [AuditBacklog.md](Engineering/AuditBacklog.md) |
| `Setup/` | Dev Windows, demo, deploy, runbook operasi, backup, email, monitoring, R2, AI tooling | [WorkspaceDirectoryMap.md](Setup/WorkspaceDirectoryMap.md), [DemoReadinessChecklist.md](Setup/DemoReadinessChecklist.md) |
| `Archive/` | Rencana iterasi 4-5, brainstorm lama, penilaian awal v0.6.0-v1.0 | Hanya referensi historis |
