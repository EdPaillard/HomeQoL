import type {
    Task, Habit, Completion, Goal, BudgetPeriod, Transaction,
    BankAccount, HomeDashboard, Alert, EnergyReading, TemperatureReading,
} from "@/types"

const BASE = import.meta.env.VITE_API_URL ?? "/api/v1"

async function req<T>(path: string, options?: RequestInit): Promise<T> {
    const res = await fetch(`${BASE}${path}`, {
        ...options,
        headers: {
            "Content-Type": "application/json",
            ...options?.headers,
        },
    })
    if (!res.ok) {
        const err = await res.json().catch(() => ({ message: res.statusText }))
        throw new Error(err.message ?? `${res.status} ${res.statusText}`)
    }
    if (res.status === 204) return undefined as T
    return res.json()
}

// ── Tasks ─────────────────────────────────────────────────────────────────────
export const tasksApi = {
    list: (params?: Record<string, string>) => {
        const qs = params ? "?" + new URLSearchParams(params) : ""
        return req<{ tasks: Task[]; count: number }>(`/tasks${qs}`)
    },
    get: (id: string) => req<Task>(`/tasks/${id}`),
    create: (body: { title: string; description?: string; priority?: number; goal_id?: string; due_date?: string; tag_ids?: string[] }) =>
        req<Task>("/tasks", { method: "POST", body: JSON.stringify(body) }),
    update: (id: string, body: Partial<{ title: string; description: string; status: string; priority: number; due_date: string | null; tag_ids: string[] }>) =>
        req<Task>(`/tasks/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
    complete: (id: string) =>
        req<Task>(`/tasks/${id}/complete`, { method: "POST" }),
    delete: (id: string) =>
        req<void>(`/tasks/${id}`, { method: "DELETE" }),
}

// ── Habits ────────────────────────────────────────────────────────────────────
export const habitsApi = {
    list: () => req<{ habits: Habit[]; count: number }>("/habits"),
    get: (id: string) => req<Habit>(`/habits/${id}`),
    create: (body: { title: string; frequency?: string; color?: string; icon?: string }) =>
        req<Habit>("/habits", { method: "POST", body: JSON.stringify(body) }),
    update: (id: string, body: Partial<Habit>) =>
        req<Habit>(`/habits/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
    complete: (id: string, date?: string, note?: string) =>
        req<Completion>(`/habits/${id}/complete`, {
            method: "POST",
            body: JSON.stringify({ date: date ?? "", note: note ?? "" }),
        }),
    uncomplete: (habitId: string, completionId: string) =>
        req<void>(`/habits/${habitId}/complete/${completionId}`, { method: "DELETE" }),
    delete: (id: string) => req<void>(`/habits/${id}`, { method: "DELETE" }),
}

// ── Goals ─────────────────────────────────────────────────────────────────────
export const goalsApi = {
    list: () => req<{ goals: Goal[]; count: number }>("/goals"),
    get: (id: string) => req<Goal>(`/goals/${id}`),
    create: (body: { title: string; description?: string; parent_id?: string; target_date?: string }) =>
        req<Goal>("/goals", { method: "POST", body: JSON.stringify(body) }),
    update: (id: string, body: Partial<{ title: string; description: string; status: string; target_date: string }>) =>
        req<Goal>(`/goals/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
    delete: (id: string) => req<void>(`/goals/${id}`, { method: "DELETE" }),
}

// ── Budget ────────────────────────────────────────────────────────────────────
export const budgetApi = {
    current: () => req<BudgetPeriod>("/budget/current"),
    period: (year: number, month: number) => req<BudgetPeriod>(`/budget/${year}/${month}`),
    transactions: (params?: Record<string, string>) => {
        const qs = params ? "?" + new URLSearchParams(params) : ""
        return req<{ transactions: Transaction[]; count: number }>(`/budget/transactions${qs}`)
    },
    createTransaction: (body: { label: string; amount_cents: number; date: string; envelope_id?: string; note?: string }) =>
        req<Transaction>("/budget/transactions", { method: "POST", body: JSON.stringify(body) }),
    assignEnvelope: (transactionId: string, envelopeId: string) =>
        req<Transaction>(`/budget/transactions/${transactionId}/envelope`, {
            method: "PATCH",
            body: JSON.stringify({ envelope_id: envelopeId }),
        }),
    accounts: () => req<{ accounts: BankAccount[] }>("/budget/accounts"),
}

// ── Home ──────────────────────────────────────────────────────────────────────
export const homeApi = {
    dashboard: () => req<HomeDashboard>("/home/dashboard"),
    alerts: (unreadOnly = false) =>
        req<{ alerts: Alert[] }>(`/home/alerts${unreadOnly ? "?unread_only=true" : ""}`),
    acknowledgeAlert: (id: string) =>
        req<void>(`/home/alerts/${id}/acknowledge`, { method: "POST" }),
    temperatureHistory: (roomId: string, from: string, to: string) =>
        req<{ readings: TemperatureReading[] }>(`/home/temperature?room_id=${roomId}&from=${from}&to=${to}`),
    energyHistory: (from: string, to: string, granularity = "daily") =>
        req<{ readings: EnergyReading[] }>(`/home/energy?from=${from}&to=${to}&granularity=${granularity}`),
}

// ── Media ──────────────────────────────────────────────────────────────────────
// Jellyfin and Transmission are proxied by nginx at /jellyfin and /transmission.
// We call their APIs directly from the browser through the nginx proxy.

const JELLYFIN_BASE = '/jellyfin'
const TRANSMISSION_BASE = '/transmission/rpc'

// Jellyfin API key — set VITE_JELLYFIN_API_KEY in .env
const JELLYFIN_KEY = import.meta.env.VITE_JELLYFIN_API_KEY ?? ''

async function jellyfinReq<T>(path: string): Promise<T> {
    const res = await fetch(`${JELLYFIN_BASE}${path}`, {
        headers: { 'X-Emby-Token': JELLYFIN_KEY },
    })
    if (!res.ok) throw new Error(`Jellyfin ${res.status}`)
    return res.json()
}

export const mediaApi = {
    // ── Jellyfin ────────────────────────────────────────────────────────────
    // Get the logged-in user's ID (needed for item queries)
    getUser: () =>
        jellyfinReq<{ Id: string; Name: string }>('/Users/Me'),

    // List movies in the library
    movies: (userId: string, params?: { StartIndex?: number; Limit?: number; SortBy?: string }) => {
        const qs = new URLSearchParams({
            IncludeItemTypes: 'Movie',
            Recursive: 'true',
            Fields: 'Overview,Genres,OfficialRating,CommunityRating,RunTimeTicks,ImageTags',
            SortBy: params?.SortBy ?? 'SortName',
            SortOrder: 'Ascending',
            StartIndex: String(params?.StartIndex ?? 0),
            Limit: String(params?.Limit ?? 50),
        })
        return jellyfinReq<{ Items: import('@/types').JellyfinItem[]; TotalRecordCount: number }>(
            `/Users/${userId}/Items?${qs}`
        )
    },

    // List TV series
    series: (userId: string, params?: { StartIndex?: number; Limit?: number }) => {
        const qs = new URLSearchParams({
            IncludeItemTypes: 'Series',
            Recursive: 'true',
            Fields: 'Overview,Genres,OfficialRating,CommunityRating,ImageTags',
            SortBy: 'SortName',
            SortOrder: 'Ascending',
            StartIndex: String(params?.StartIndex ?? 0),
            Limit: String(params?.Limit ?? 50),
        })
        return jellyfinReq<{ Items: import('@/types').JellyfinItem[]; TotalRecordCount: number }>(
            `/Users/${userId}/Items?${qs}`
        )
    },

    // Poster image URL helper — call directly in <img src={mediaApi.poster(item)} />
    poster: (item: import('@/types').JellyfinItem, size = 300) =>
        item.ImageTags?.Primary
            ? `${JELLYFIN_BASE}/Items/${item.Id}/Images/Primary?maxHeight=${size}&quality=90`
            : null,

    // ── Transmission ────────────────────────────────────────────────────────
    // Transmission uses a session token anti-CSRF system.
    // The proxy passes X-Transmission-Session-Id through (see nginx.conf).
    torrents: async (): Promise<{ torrents: import('@/types').Torrent[] }> => {
        let sessionId = ''
        const body = JSON.stringify({
            method: 'torrent-get',
            arguments: {
                fields: ['id', 'name', 'status', 'percentDone', 'rateDownload',
                    'rateUpload', 'totalSize', 'eta', 'downloadDir', 'error', 'errorString'],
            },
        })

        // First attempt — may get 409 with the session ID in the header
        let res = await fetch(TRANSMISSION_BASE, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-Transmission-Session-Id': sessionId },
            body,
        })
        if (res.status === 409) {
            sessionId = res.headers.get('X-Transmission-Session-Id') ?? ''
            res = await fetch(TRANSMISSION_BASE, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json', 'X-Transmission-Session-Id': sessionId },
                body,
            })
        }
        const data = await res.json()
        return { torrents: data.arguments?.torrents ?? [] }
    },

    removeTorrent: async (id: number, deleteData = false): Promise<void> => {
        await fetch(TRANSMISSION_BASE, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                method: 'torrent-remove',
                arguments: { ids: [id], 'delete-local-data': deleteData },
            }),
        })
    },
}