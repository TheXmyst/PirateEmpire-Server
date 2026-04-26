# ⚓ Sea-Dogs — Real-Time MMO Pirate Strategy Server

> A full-stack real-time multiplayer game built entirely in Go — from the authoritative game server to the native desktop client.

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-4169E1?style=flat-square&logo=postgresql)
![Ebiten](https://img.shields.io/badge/Client-Ebiten-FF6B35?style=flat-square)
![Build](https://img.shields.io/badge/build-passing-brightgreen?style=flat-square)
![Status](https://img.shields.io/badge/status-alpha--ready-blue?style=flat-square)
![Architecture](https://img.shields.io/badge/architecture-clean-success?style=flat-square)

---

## What This Is

Sea-Dogs is a **real-time MMO pirate strategy game** where players manage islands, build fleets, recruit captains, and engage in PvE/PvP naval combat — all synchronized across a persistent server.

This is a **solo-built, production-grade backend** demonstrating:
- Authoritative game server with complex concurrent state management
- 12 interconnected game systems with clean separation of concerns
- ACID transaction safety across all economy-critical operations
- Real-time multi-player synchronization without WebSockets (polling + server authority)

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENTS (Ebiten)                        │
│   World Map · Island UI · Fleet Manager · Tavern · Chat         │
└────────────────────────┬────────────────────────────────────────┘
                         │ REST API (40+ endpoints)
┌────────────────────────▼────────────────────────────────────────┐
│                    SERVER (Go + Echo v4)                         │
│                                                                  │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐  │
│  │  API Layer  │  │ Economy Layer │  │    Domain / Models     │  │
│  │  handlers/  │─▶│  gacha.go    │  │  25+ entity types      │  │
│  │  pvp / pve  │  │  naval.go    │  │  Fleet · Ship ·        │  │
│  │  cargo /    │  │  tech.go     │  │  Captain · Island ·    │  │
│  │  nav / auth │  │  shipping.go │  │  Building · Resource   │  │
│  └─────────────┘  └──────────────┘  └────────────────────────┘  │
│                                                                  │
│  ┌──────────────────────────┐  ┌─────────────────────────────┐  │
│  │     Repository (GORM)    │  │    Config (JSON-driven)     │  │
│  │  SELECT FOR UPDATE locks │  │  buildings · ships · tech   │  │
│  │  ACID transactions       │  │  Balance without code edit  │  │
│  └─────────────┬────────────┘  └─────────────────────────────┘  │
└────────────────┼────────────────────────────────────────────────┘
                 │
┌────────────────▼────────────────────────────────────────────────┐
│                     PostgreSQL (~25 tables)                      │
│  Islands · Fleets · Ships · Captains · Buildings · Tech ·       │
│  ResourceNodes · SeaZones · Wallets · Migrations                │
└─────────────────────────────────────────────────────────────────┘
```

---

## Game Systems (12 Implemented)

| System | Description | Status |
|--------|-------------|--------|
| **Naval Combat** | Stat-based simulation: ATK/DEF/HP, morale, captain passives, DR cap | ✅ |
| **Gacha & Captains** | Double-tier pity (80 legendary / 10 rare), shard wallets, daily caps | ✅ |
| **Economy & Resources** | 7 resource types, multi-factor production formula, storage caps | ✅ |
| **Fleet Management** | 9-state machine (Idle→Chasing→TravelingToAttack→Returning...) | ✅ |
| **PvP Interception** | Real-time pursuit, combat initiation, peace shields, beginner protection | ✅ |
| **PvE Combat** | NPC fleet generation, loot tables, rarity scaling | ✅ |
| **Building & Construction** | 9 building types, exponential cost scaling, single build queue | ✅ |
| **Tech Tree** | 20+ techs, dependency graph, TH-gated research, multiplicative bonuses | ✅ |
| **Matchmaking & Auth** | Sea zones (50 players/sea), collision avoidance, bcrypt auth | ✅ |
| **Militia Recruitment** | 3 crew types (Warriors/Archers/Gunners), queue system, ship assignment | ✅ |
| **World Map & Resources** | Shader-animated water, procedural resource nodes, 1h TTL cache | ✅ |
| **Chat** | Sea-scoped persistent chat, throttled (1 msg/2s), scroll history | ✅ |

---

## Technical Highlights

### Concurrency & Safety

Race conditions were a real challenge in this project — here's how they were solved:

**Gacha Pity — SELECT FOR UPDATE**
```go
// Without this: 2 simultaneous x10 summons could both read pity=5,
// resulting in pity=15 instead of 25. Legendary guarantee broken.
tx.Set("gorm:query_option", "FOR UPDATE").First(&playerWithPity, "id = ?", playerID)
```

**Checkpoint Throttling — 5s Interval**
```go
// Without this: GetStatus (polled 1-2x/sec) saved Island on every call
// With 50 concurrent players: exponential DB contention
const StatusCheckpointInterval = 5 * time.Second

if timeSinceLastCheckpoint >= StatusCheckpointInterval {
    island.LastCheckpointSavedAt = &now
    // Save to DB — at most once per 5 seconds
}
```

**Economy Atomicity — ACID Transactions**
```go
// ExchangeShards: if any wallet save fails, entire exchange rolls back
// Player never loses shards without receiving tickets
tx := db.Begin()
defer func() {
    if r := recover(); r != nil { tx.Rollback() }
}()
for i := range wallets {
    tx.Save(&wallets[i])
}
tx.Save(&island)    // Add tickets
tx.Commit()         // All-or-nothing
```

---

### Naval Combat Engine

Combat uses a multi-factor simulation computed server-side:

```
EffectiveDR = baseShipDR + (captainPassive × 0.02) + (armorTech × 0.001)
              └── Capped at 0.90 (no immortality)

Damage = (AttackerATK - DefenderDEF) × (1 - EffectiveDR)
```

**Morale System** — Applied before damage rolls:
```go
// Rum shortage penalty: -20 morale
// Captain passives: absolute_morale_floor, terror_engagement, opening_enemy_morale_damage
// Delta tiers: |ΔM| 0-4 = 0% | 5-9 = 5% | 10-19 = 10% | 20-29 = 18% | 30-39 = 28% | 40+ = 40%
// Winner gets ATK × (1 + bonusPercent) and DEF × (1 + bonusPercent)
```

---

### Dynamic Music System (SeaDMS)

A custom adaptive audio engine with 6 independent stems:

```
Stems: [Vocals] [Drums] [Bass] [Guitar] [Percussion] [Synth]

Calm Mode  (Exploration): Vocals✓  Guitar✓  Percussion✓  Synth✓  | Drums✗  Bass✗
Combat Mode (Battle):     Drums✓   Bass✓   | Vocals↓85%  Guitar↓85%  Percussion↓85%

Transitions: Linear fade over 1 second, per-stem volume automation
Trigger:     Automatic based on IsCombatActive() game state
```

---

### Configuration-Driven Design

Game balance is managed through JSON — no code changes required:

```json
// server/configs/buildings.json — excerpt
{
  "lumber_mill": {
    "base_production": 12,
    "cost_growth_factor": 1.39,
    "max_level": 10,
    "resource": "Wood"
  }
}
```

Buildings, ships, and technologies are all config-driven. Rebalancing is a JSON edit.

---

## Project Structure

```
Sea-Dogs/
├── server/                    # Go backend (Echo v4 + GORM)
│   ├── cmd/main.go
│   └── internal/
│       ├── api/               # 40+ REST endpoints
│       │   ├── handlers.go    # Core handlers (~3000 LOC)
│       │   ├── handlers_pvp.go
│       │   ├── handlers_pve.go
│       │   ├── handlers_navigation.go
│       │   └── handlers_cargo.go
│       ├── economy/           # Game logic (~800 LOC)
│       │   ├── naval.go       # Combat simulation
│       │   ├── gacha.go       # Summon & pity
│       │   ├── tech.go        # Tech tree
│       │   └── engagement_morale.go
│       ├── domain/            # Entity types (25+ models)
│       ├── repository/        # GORM data access
│       └── auth/              # Bcrypt authentication
│
├── client/                    # Go desktop client (Ebiten)
│   ├── cmd/
│   │   ├── main.go            # Game loop & state routing
│   │   ├── login_ui.go
│   │   └── world_map_ui.go
│   └── internal/game/         # UI components
│       ├── fleet_ui.go
│       ├── pvp_ui.go
│       ├── construction_ui.go
│       ├── tech_ui.go
│       └── sea_dms_music.go   # Dynamic music system
│
├── server/configs/
│   ├── buildings.json          # 9 building types
│   ├── tech.json               # 20+ technologies
│   └── ships.json              # Ship templates
│
└── tools/
    └── modelguard/             # Developer utilities
```

---

## Stack

| Layer | Technology |
|-------|------------|
| Backend language | Go 1.25 |
| HTTP framework | Echo v4 |
| ORM | GORM |
| Database | PostgreSQL |
| Client | Ebiten (2D game engine) |
| Auth | bcrypt + session tokens |
| Config | JSON (buildings, ships, tech) |
| Workspace | Go Workspaces (multi-module) |

---

## What This Demonstrates

- **Concurrent state management** — race conditions identified and resolved (SELECT FOR UPDATE, throttled checkpointing)
- **ACID transaction design** — all economy operations are atomic and rollback-safe
- **Game systems architecture** — 12 interconnected systems with clean layer separation
- **Config-driven balance** — game parameters externalized for iteration without deploys
- **Authoritative server design** — all combat, economy, and navigation resolved server-side

---

## Running Locally

```bash
# Prerequisites: Go 1.25+, PostgreSQL

# 1. Clone
git clone https://github.com/TheXmyst/Sea-Dogs.git
cd Sea-Dogs

# 2. Configure database
cp server/.env.example server/.env
# Edit DATABASE_URL in .env

# 3. Run server
cd server && go run ./cmd

# 4. Run client (separate terminal)
cd client && go run ./cmd
```

---

*Solo project — backend, client, game design, and audio system all built by one developer.*
