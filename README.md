# Benchmarq

<div align="center">

**High-Integrity Online Examination & Automated Proctoring Platform**

A modern, secure monorepo platform designed for delivering high-stakes exams, managing candidates, and enforcing lockdown proctoring across web and desktop environments.

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![Next.js](https://img.shields.io/badge/Next.js-16-black?style=flat&logo=next.js)](https://nextjs.org)
[![Electron](https://img.shields.io/badge/Electron-45-47848F?style=flat&logo=electron)](https://www.electronjs.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-336791?style=flat&logo=postgresql)](https://www.postgresql.org)
[![Tailwind CSS](https://img.shields.io/badge/TailwindCSS-v4-38B2AC?style=flat&logo=tailwind-css)](https://tailwindcss.com)

</div>

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Core Features](#core-features)
  - [Secure Desktop Kiosk (`desktop`)](#1-secure-desktop-kiosk-desktop)
  - [Backend API Engine (`server`)](#2-backend-api-engine-server)
  - [Web Portal & Authoring (`client-web`)](#3-web-portal--authoring-client-web)
- [Monorepo Structure](#monorepo-structure)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
  - [1. Database Setup](#1-database-setup)
  - [2. Server Setup](#2-server-setup)
  - [3. Web Client Setup](#3-web-client-setup)
  - [4. Desktop Exam Client Setup](#4-desktop-exam-client-setup)
- [Exam Launch & Deep Link Flow](#exam-launch--deep-link-flow)
- [Environment Variables](#environment-variables)
  - [Server Configuration](#server-environment-variables)
  - [Web Client Configuration](#web-client-environment-variables)
  - [Desktop App Configuration](#desktop-app-build-configuration)
- [Building & Packaging](#building--packaging)
- [License](#license)

---

## Overview

**Benchmarq** solves the challenges of modern academic and recruitment assessments by coupling an intuitive administrative and candidate portal with a hardened desktop proctoring client. 

Organizations can author exams with multiple sections, custom scoring policies, and randomized question banks, assign tests to candidates via secure invitation links, and monitor real-time attempt progress. When taking an exam, candidates are transitioned via deep link into a secure lockdown environment that prevents code injection, monitors multi-display configurations, blocks unauthorized background processes, and captures proctoring snapshots.

---

## Architecture

```
                    ┌─────────────────────────────────┐
                    │       Examiner / Admin          │
                    │   (client-web / Dashboard)      │
                    └────────────────┬────────────────┘
                                     │
                                     ▼
┌──────────────────┐         ┌───────────────┐         ┌──────────────────────┐
│  Candidate Web   │         │               │         │   Candidate Desktop  │
│      Portal      ├────────►│  Go REST API  │◄────────┤      Exam Kiosk      │
│  (client-web)    │         │   (server)    │         │      (desktop)       │
└────────┬─────────┘         └───────┬───────┘         └──────────┬───────────┘
         │                           │                            │
         │   benchmarq://launch?code │                            │ Webcam / Screen
         └───────────────────────────┼────────────────────────────┘ Snapshots &
                                     ▼                              Violation Telemetry
                             ┌───────────────┐
                             │  PostgreSQL   │
                             │  (bechmarqdb) │
                             └───────────────┘
```

The system is composed of three services:

1. **`server`**: Go REST API powered by Chi, PostgreSQL (via `pgx`), background sweepers, and an asynchronous worker-pooled SMTP mailer.
2. **`desktop`**: Hardened Electron desktop client providing strict kiosk lockdown, process monitoring, VM detection, anti-tampering fuses, and scheduled webcam/screen capture.
3. **`client-web`**: Next.js 16 (React 19) web application providing the staff workspace (authoring, scoring, assignment management, proctoring review) and candidate onboarding/fallback portal.

---

## Core Features

### 1. Secure Desktop Kiosk (`desktop`)

- **Full-Screen Lockdown**: Restricts navigation, suppresses destructive shortcuts (`Cmd/Ctrl+R`, `F5`, `F11`, `F12`, `Alt+Tab`, inspect element, and window minimizations), and immediately detects focus loss or fullscreen exit violations.
- **Process & Anti-Cheat Enforcement**: Continuously detects and closes unauthorized background processes (e.g., AnyDesk, TeamViewer, Parsec, RustDesk, VNC, ScreenConnect, remote desktop tools, virtual cameras, and AI screen-reading assistants).
- **Integrity & Anti-Tamper Protection**: Utilizes Electron fuses to disable Node debugging switches (`--inspect`, `--remote-debugging-port`), blocks Node environment variable overrides, and seals configuration into an integrity-checked `app.asar`.
- **Hardware & VM Detection**: Verifies single-monitor compliance and alerts on virtual machine environments (QEMU, VMware, VirtualBox, Hyper-V, Parallels).
- **Proctoring Snapshots**: Scheduled capture of candidate webcam frames and desktop screenshots sent directly to secure storage.
- **Native Protocol Handler**: Integrates seamlessly with `benchmarq://launch?code=<access_code>` deep links from the web portal.

### 2. Backend API Engine (`server`)

- **Modular Clean Architecture**: Divided into dedicated domain modules (`assignments`, `attempts`, `auth`, `exams`, `organizations`, `questions`).
- **Robust Authentication**: JWT access token paired with secure HTTP-only rotating refresh cookies, email verification workflows, and secure password reset tokens.
- **Multi-Tenant Organization & RBAC**: Granular role-based permissions (Owner, Admin, Member) for exam creation, question banks, candidate review, and org invitations.
- **Exam & Section Authoring**: Hierarchical exam structure supporting multiple sections, section-level default scoring, custom question points, negative marking, and duration constraints.
- **Live Attempt Runtime**: Atomic session validation, heartbeat pings, real-time response auto-saving, violation telemetry logging, and snapshot storage.
- **Background Sweeper Service**: Automatic garbage collection and settlement of orphaned, expired, or disconnected exam attempts.
- **Asynchronous Mailer Pool**: Concurrent worker-queue architecture for sending invitation, verification, and notification emails via SMTP.

### 3. Web Portal & Authoring (`client-web`)

- **Modern Tech Stack**: Next.js 16 with App Router, React 19, Tailwind CSS v4, MUI Material components, and TanStack React Query for snappy server-state synchronization.
- **Staff Dashboard**:
  - Exam builder with section ordering, timing rules, and submission constraints.
  - Question editor with multi-choice choices and weighted scoring.
  - Candidate batch assignment and invitation tracking.
  - Attempt inspector showing time spent, status, scores, and logged proctoring flags.
- **Candidate Experience**:
  - Clean waiting room with pre-flight checklist and deep-link launcher to the desktop app.
  - In-browser candidate fallback interface with real-time timer countdown, question grid navigator, question state badges, and submission confirmations.

---

## Monorepo Structure

```text
benchmarq/
├── client-web/                 # Next.js 16 Web Application
│   ├── app/                    # Next.js App Router (Dashboard, Exams, Auth, Portal)
│   ├── components/             # Reusable UI & feature components (ExamShell, WaitingRoom, etc.)
│   ├── context/                # React context providers (AuthContext, etc.)
│   ├── hooks/                  # Custom hooks
│   └── lib/                    # API clients, Axios interceptors, TanStack Query configs
│
├── desktop/                    # Electron Secure Exam Client
│   ├── build/                  # Build scripts, Electron fuses, macOS entitlements
│   ├── scripts/                # Build-time configuration helpers
│   ├── src/
│   │   ├── capture/            # Webcam & screen capture renderers
│   │   ├── main/               # Main process, lockdown coordinator, snapshot manager
│   │   │   └── integrity/      # Anti-tamper, VM detection, and process scanners
│   │   └── preload/            # Context bridges and secure IPC preloads
│   └── test/                   # Unit & integrity tests
│
└── server/                     # Go Backend API
    ├── cmd/
    │   ├── api/                # Main HTTP API entrypoint & graceful shutdown
    │   └── migrate/            # CLI migration runner
    ├── internal/
    │   ├── authz/              # RBAC middleware & access authorization
    │   ├── config/             # Environment configuration parsing
    │   ├── database/           # pgx connection pool & transaction utilities
    │   ├── mailer/             # Asynchronous SMTP worker group
    │   ├── middleware/         # CORS, logging, panic recovery, security headers
    │   ├── modules/            # Domain logic (auth, exams, questions, attempts, etc.)
    │   └── router/             # Chi router assembly & endpoint registration
    ├── migrations/             # SQL migration files (up/down)
    └── pkg/                    # Shared utilities (logger, request/response helpers)
```

---

## Prerequisites

Before running Benchmarq locally, ensure you have:

- **Go**: `1.24+` (or compatible with Go 1.27 toolchain)
- **Node.js**: `v20.0.0+` (LTS recommended) and `npm`
- **Docker & Docker Compose**: For local PostgreSQL database
- **Operating System**: macOS, Linux, or Windows (Desktop lockdown features support all three OS platforms)

---

## Quick Start

### 1. Database Setup

Spin up the local PostgreSQL instance using Docker Compose:

```bash
cd server
docker-compose up -d
```

This starts PostgreSQL 15 on port `5433` (database: `bechmarqdb`, user: `user`, password: `password`).

---

### 2. Server Setup

1. Copy the example environment file and configure variables as needed:
   ```bash
   cd server
   cp .env.example .env
   ```

2. Run the database migrations:
   ```bash
   go run cmd/migrate/main.go
   ```

3. Start the API server:
   ```bash
   # Standard execution
   go run cmd/api/main.go

   # Or with live reload via Air (if installed)
   air
   ```

The backend API will start at [http://localhost:8000](http://localhost:8000).  
Check health status: [http://localhost:8000/health](http://localhost:8000/health).

---

### 3. Web Client Setup

1. Navigate to the web directory and install dependencies:
   ```bash
   cd client-web
   npm install
   ```

2. Configure the client environment (defaults to port 8000):
   ```bash
   cp .env.example .env.local
   ```

3. Launch the development server:
   ```bash
   npm run dev
   ```

Open [http://localhost:3000](http://localhost:3000) in your browser.

---

### 4. Desktop Exam Client Setup

1. Navigate to the desktop directory and install dependencies:
   ```bash
   cd desktop
   npm install
   ```

2. Run the desktop app in development mode:
   ```bash
   npm run dev
   ```

3. Test deep linking directly with a candidate code:
   ```bash
   npm run launch:dev -- --code="YOUR_EXAM_ACCESS_CODE"
   ```

---

## Exam Launch & Deep Link Flow

Benchmarq uses a custom deep-link scheme to seamlessly bridge the candidate web portal with the secure desktop application:

```
[Candidate accepts assignment / enters portal]
                    │
                    ▼
[Web checks desktop readiness & shows Launch button]
                    │
                    ▼
[Deep link invoked: benchmarq://launch?code=ABCD123]
                    │
                    ▼
[Benchmarq Exam Client boots in kiosk lockdown]
                    │
                    ├── Validates candidate attempt session
                    ├── Performs integrity & process checks
                    ├── Starts webcam/screen snapshot capture
                    └── Presents the locked-down exam shell
```

---

## Environment Variables

### Server Environment Variables

Defined in `server/.env` (reference: `server/.env.example`):

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8000` | HTTP port for the API server |
| `ENV` | `development` | Runtime environment (`development`, `production`) |
| `DATABASE_URL` | `postgres://user:password@localhost:5433/bechmarqdb?sslmode=disable` | PostgreSQL connection string |
| `ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:5173` | Allowed CORS origins (comma-separated) |
| `FRONTEND_URL` | `http://localhost:3000` | Base URL of the candidate/admin web portal |
| `DESKTOP_LAUNCH_URL`| `benchmarq://launch` | Protocol prefix for launching the desktop app |
| `SNAPSHOT_DIR` | `storage/snapshots` | Directory on disk where proctoring snapshots are saved |
| `JWT_SECRET` | `benchmarq_super_secret...` | Secret key used to sign access/refresh tokens |
| `JWT_ACCESS_EXPIRY_MINUTES` | `15` | Expiry duration for JWT access tokens |
| `JWT_REFRESH_EXPIRY_DAYS` | `7` | Expiry duration for refresh tokens |
| `COOKIE_SECURE` | `false` | Enable `Secure` flag for cookies in production (HTTPS) |
| `SMTP_HOST` | `localhost` | SMTP host for sending emails |
| `SMTP_PORT` | `1025` | SMTP port |
| `EMAIL_WORKERS` | `5` | Number of concurrent background email workers |

### Web Client Environment Variables

Defined in `client-web/.env.local`:

| Variable | Default | Description |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8000/api/v1` | Base API URL pointing to the backend |

### Desktop App Build Configuration

Packaged builds lock their server targets at build time to prevent unauthorized redirect attacks. Variables can be passed when packaging:

| Variable | Default | Description |
|---|---|---|
| `BENCHMARQ_APP_URL` | `http://localhost:3000` | URL of the candidate exam page loaded by the kiosk |
| `BENCHMARQ_API_URL` | `http://localhost:8000/api/v1` | URL of the backend API runtime endpoints |

---

## Building & Packaging

### Building the Desktop Application

Desktop packaging uses `electron-builder` and `electron/fuses` to lock down developer features in release binaries:

```bash
cd desktop

# Build macOS DMG (.dmg / .zip)
npm run dist:mac

# Build Windows Installer (.exe)
npm run dist:win

# Build all configured platform targets
npm run dist
```

### Building the Web Application

```bash
cd client-web
npm run build
npm run start
```

### Building the Go Backend

```bash
cd server
CGO_ENABLED=0 go build -o bin/benchmarq-api ./cmd/api
```

---

## License

This repository is private and proprietary. All rights reserved.
