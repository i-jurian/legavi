# 06 - Release Orchestration

**Update history**

- 2026-05-09: Initial draft

---

Inactivity-detection state machine and the release process. The state machine must be deterministic, recoverable, and verifiable.

## 1. State diagram

```mermaid
stateDiagram-v2
    [*] --> ACTIVE
    ACTIVE --> REMINDED_SOFT: inactive >= soft_after_days (7d)
    REMINDED_SOFT --> REMINDED_FIRM: inactive >= firm_after_days (14d)
    REMINDED_FIRM --> REMINDED_FINAL: inactive >= final_after_days (30d)
    REMINDED_FINAL --> COOLING: final reminder sent
    COOLING --> FINAL_HOLD: cooling_hours elapsed (48h)
    FINAL_HOLD --> RELEASED: final_hold_hours elapsed (24h)

    REMINDED_SOFT --> ACTIVE: checkin
    REMINDED_FIRM --> ACTIVE: checkin
    REMINDED_FINAL --> ACTIVE: checkin
    COOLING --> ACTIVE: checkin
    FINAL_HOLD --> ACTIVE: checkin or false-positive

    RELEASED --> [*]
```

## 2. State definitions

| State            | Description                                                                            | Owner can cancel?                                               | Recipients receive? |
| ---------------- | -------------------------------------------------------------------------------------- | --------------------------------------------------------------- | ------------------- |
| `ACTIVE`         | Normal operation.                                                                      | N/A                                                             | No                  |
| `REMINDED_SOFT`  | Soft reminder sent (configurable cadence; default 7 days inactive).                    | Yes (checkin)                                                   | No                  |
| `REMINDED_FIRM`  | Firm reminder sent (default 14 days inactive).                                         | Yes (checkin)                                                   | No                  |
| `REMINDED_FINAL` | Final reminder sent (default 30 days inactive).                                        | Yes (checkin)                                                   | No                  |
| `COOLING`        | Final reminder elapsed without check-in. 48-hour cooling period before formal release. | Yes (checkin)                                                   | Notification only   |
| `FINAL_HOLD`     | Cooling expired. 24-hour final hold during which owner can still flag false-positive.  | Yes (false-positive flag, which also revokes any tokens issued) | Notification only   |
| `RELEASED`       | Final hold expired. Recipients can now retrieve their assigned data.                   | No (terminal)                                                   | Yes                 |

## 3. Transition rules

The state machine is a pure function `nextState(current, releaseState, offsets, now) -> nextState`. All transitions are derived from this function deterministically.

**Precondition for evaluation:** an owner is only evaluated by the scheduler if they have at least one entry currently assigned to a contact. Owners with zero contact-assigned entries stay in `ACTIVE` regardless of inactivity (no one to release to). The state machine engages the moment they make their first contact assignment, starting from their current `last_checkin_at` (reset by the assignment action itself).

### 3.1 `ACTIVE` → `REMINDED_SOFT`

When `now - last_checkin_at >= soft_after_days`. Worker sends a soft reminder email.

### 3.2 `REMINDED_SOFT` → `REMINDED_FIRM`

When `now - last_checkin_at >= firm_after_days`. Worker sends a firm reminder.

### 3.3 `REMINDED_FIRM` → `REMINDED_FINAL`

When `now - last_checkin_at >= final_after_days`. Worker sends the final reminder.

### 3.4 `REMINDED_FINAL` → `COOLING`

Immediately after the final reminder is sent. `cooling_started_at` is set to `now`. Worker emails each designated recipient: "X has not checked in. Release is in cooling period."

### 3.5 `COOLING` → `FINAL_HOLD`

When `now - cooling_started_at >= cooling_hours` (default 48 hours). Worker emails owner one more time: "Cooling period expired. 24-hour final hold begins. Flag false-positive in the app to cancel." `final_hold_until` is set to `now + final_hold_hours`.

### 3.6 `FINAL_HOLD` → `RELEASED`

When `now >= final_hold_until` AND `false_positive_flag = false`. Worker emails each contact who has at least one entry assigned to them with their recovery link. Contacts with no assignments receive nothing.

### 3.7 Any state → `ACTIVE` (check-in)

Owner makes an authenticated request (any endpoint) before `RELEASED`. `last_checkin_at` is updated; state transitions to `ACTIVE` if it was anywhere in `REMINDED_*`, `COOLING`, or `FINAL_HOLD`. Audit log records the cancellation event.

### 3.8 `FINAL_HOLD` → `ACTIVE` (false-positive flag)

Owner explicitly flags false-positive. Same as 3.7 plus: any recovery tokens already issued to recipients are invalidated server-side.

### 3.9 No transition out of `RELEASED`

`RELEASED` is terminal. If a release fires but the owner is still active afterwards, recovery requires manual intervention (re-registration, re-onboarding contacts, fresh vault).

## 4. Idempotency

The scheduler ticks every 60 seconds. Each tick evaluates the state machine for every user. State transitions emit jobs to the queue (with deduplication keys derived from `user_id + transition_id + sequence`). Reprocessing the same tick produces no duplicate emails.

Worker jobs are idempotent: each event has a unique key. Re-running a job that has already completed is a no-op.

## 5. Crash recovery

The scheduler is single-leader via Postgres advisory lock (`pg_try_advisory_lock(LGV_SCHEDULER_LEADER_LOCK_ID)`). On crash, another scheduler instance acquires the lock and resumes from the next tick.

Mid-transition crash recovery: state transitions are written transactionally with the corresponding job emission. Either both the new state AND the queued job are committed, or neither. On recovery, the scheduler observes the consistent state and continues.

## 6. Time-fast-forward (test mode)

When `LGV_TEST_MODE=true`, `POST /api/v1/test/fast-forward` advances a per-user clock offset stored in the database; the scheduler reads `clock.now() = real_now + offset` for that user and processes all transitions that would have occurred during the interval. Used by E2E tests to simulate 30-day inactivity in seconds.

## 7. Audit log integration

Every state transition produces an audit log entry with `event_type: release_state_change` and payload `{ from, to, reason }` where `reason` is `scheduler_tick`, `checkin`, or `false_positive`. The owner's browser signs a checkpoint after every state-change event.
