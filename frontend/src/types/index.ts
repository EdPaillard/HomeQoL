// ── Tasks ──────────────────────────────────────────────────────────────────────
export type TaskStatus = "todo" | "in_progress" | "done" | "cancelled"
export type Priority = 1 | 2 | 3

export interface Tag {
    id: string
    name: string
    color: string
}

export interface Task {
    id: string
    goal_id?: string
    title: string
    description: string
    status: TaskStatus
    priority: Priority
    due_date?: string
    completed_at?: string
    tags: Tag[]
    created_at: string
    updated_at: string
}

// ── Habits ─────────────────────────────────────────────────────────────────────
export type Frequency = "daily" | "weekly"

export interface Habit {
    id: string
    title: string
    description: string
    frequency: Frequency
    scheduled_days: number[]
    target_per_week?: number
    color: string
    icon: string
    is_active: boolean
    current_streak: number
    longest_streak: number
    last_completed_date?: string
    completed_today: boolean
}

export interface Completion {
    id: string
    habit_id: string
    completed_date: string
    note: string
}

// ── Goals ──────────────────────────────────────────────────────────────────────
export type GoalStatus = "active" | "completed" | "abandoned"

export interface Goal {
    id: string
    parent_id?: string
    title: string
    description: string
    status: GoalStatus
    target_date?: string
    completed_at?: string
    milestones: Goal[]
    task_count: number
    done_count: number
    progress_pct: number
    created_at: string
    updated_at: string
}

// ── Budget ─────────────────────────────────────────────────────────────────────
export interface Envelope {
    id: string
    period_id: string
    template_id?: string
    name: string
    budgeted_cents: number
    spent_cents: number
    budgeted_euros: number
    spent_euros: number
    remaining_cents: number
    percent_used: number
    color: string
    icon: string
    sort_order: number
}

export interface PeriodSummary {
    total_budgeted_cents: number
    total_spent_cents: number
    total_budgeted_euros: number
    total_spent_euros: number
    remaining_cents: number
    remaining_euros: number
}

export interface BudgetPeriod {
    id: string
    year: number
    month: number
    is_closed: boolean
    envelopes: Envelope[]
    summary: PeriodSummary
    created_at: string
}

export interface Transaction {
    id: string
    account_id?: string
    envelope_id?: string
    external_id?: string
    label: string
    clean_label: string
    amount_cents: number
    amount_euros: number
    currency: string
    transaction_date: string
    category_rule?: string
    source: "bank" | "manual"
    note: string
}

export interface BankAccount {
    id: string
    external_id: string
    label: string
    type: string
    balance_cents: number
    balance_euros: number
    currency: string
    iban?: string
    last_synced_at?: string
}

// ── Home monitor ───────────────────────────────────────────────────────────────
export interface Room {
    id: string
    name: string
    external_id?: string
}

export interface TemperatureReading {
    id: string
    room_id: string
    room_name: string
    celsius: number
    source: "interior" | "exterior"
    recorded_at: string
}

export interface ShutterReading {
    id: string
    room_id: string
    room_name: string
    position: number
    recorded_at: string
}

export interface SunlightReading {
    id: string
    lux: number
    recorded_at: string
}

export interface EnergyReading {
    id: string
    reading_date: string
    reading_hour?: number
    kwh: number
    granularity: "daily" | "hourly"
}

export interface Alert {
    id: string
    type: string
    severity: "info" | "warning" | "critical"
    title: string
    body: string
    source_table?: string
    source_id?: string
    acknowledged_at?: string
    created_at: string
}

export interface HomeDashboard {
    rooms: Room[]
    latest_temperatures: TemperatureReading[]
    latest_shutters: ShutterReading[]
    latest_sunlight?: SunlightReading
    today_kwh?: number
    month_kwh?: number
    unread_alerts: number
    generated_at: string
}

// ── Media ──────────────────────────────────────────────────────────────────────

export interface JellyfinItem {
    Id: string
    Name: string
    Type: 'Movie' | 'Series' | 'Season' | 'Episode'
    ProductionYear?: number
    OfficialRating?: string
    CommunityRating?: number
    Overview?: string
    Genres?: string[]
    RunTimeTicks?: number          // in ticks (1 tick = 100ns)
    ImageTags?: { Primary?: string }
    BackdropImageTags?: string[]
}

export interface Torrent {
    id: number
    name: string
    status: number                 // Transmission status codes 0-6
    percentDone: number            // 0.0 – 1.0
    rateDownload: number           // bytes/s
    rateUpload: number             // bytes/s
    totalSize: number              // bytes
    eta: number                    // seconds, -1 = unknown
    downloadDir: string
    error: number
    errorString: string
}