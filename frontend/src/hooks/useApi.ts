import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { tasksApi, habitsApi, goalsApi, budgetApi, homeApi } from "@/api/client"

// ── Tasks ─────────────────────────────────────────────────────────────────────
export function useTasks(params?: Record<string, string>) {
    return useQuery({
        queryKey: ["tasks", params],
        queryFn: () => tasksApi.list(params),
    })
}

export function useTodayTasks() {
    return useQuery({
        queryKey: ["tasks", "today"],
        queryFn: () => tasksApi.list({ status: "todo", due_today: "true" }),
    })
}

export function useCompleteTask() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: (id: string) => tasksApi.complete(id),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["tasks"] }),
    })
}

export function useCreateTask() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: tasksApi.create,
        onSuccess: () => qc.invalidateQueries({ queryKey: ["tasks"] }),
    })
}

export function useUpdateTask() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: ({ id, ...body }: Parameters<typeof tasksApi.update>[1] & { id: string }) =>
            tasksApi.update(id, body),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["tasks"] }),
    })
}

export function useDeleteTask() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: (id: string) => tasksApi.delete(id),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["tasks"] }),
    })
}

// ── Habits ────────────────────────────────────────────────────────────────────
export function useHabits() {
    return useQuery({
        queryKey: ["habits"],
        queryFn: () => habitsApi.list(),
    })
}

export function useCompleteHabit() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: ({ id, date }: { id: string; date?: string }) =>
            habitsApi.complete(id, date),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["habits"] }),
    })
}

export function useUncompleteHabit() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: ({ habitId, completionId }: { habitId: string; completionId: string }) =>
            habitsApi.uncomplete(habitId, completionId),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["habits"] }),
    })
}

export function useCreateHabit() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: habitsApi.create,
        onSuccess: () => qc.invalidateQueries({ queryKey: ["habits"] }),
    })
}

// ── Goals ─────────────────────────────────────────────────────────────────────
export function useGoals() {
    return useQuery({
        queryKey: ["goals"],
        queryFn: () => goalsApi.list(),
    })
}

export function useCreateGoal() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: goalsApi.create,
        onSuccess: () => qc.invalidateQueries({ queryKey: ["goals"] }),
    })
}

export function useUpdateGoal() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: ({ id, ...body }: Parameters<typeof goalsApi.update>[1] & { id: string }) =>
            goalsApi.update(id, body),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["goals"] }),
    })
}

// ── Budget ────────────────────────────────────────────────────────────────────
export function useCurrentBudget() {
    return useQuery({
        queryKey: ["budget", "current"],
        queryFn: () => budgetApi.current(),
        staleTime: 60_000,
    })
}

export function useTransactions(params?: Record<string, string>) {
    return useQuery({
        queryKey: ["transactions", params],
        queryFn: () => budgetApi.transactions(params),
    })
}

export function useBankAccounts() {
    return useQuery({
        queryKey: ["accounts"],
        queryFn: () => budgetApi.accounts(),
        staleTime: 5 * 60_000,
    })
}

export function useCreateTransaction() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: budgetApi.createTransaction,
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: ["transactions"] })
            qc.invalidateQueries({ queryKey: ["budget"] })
        },
    })
}

// ── Home ──────────────────────────────────────────────────────────────────────
export function useHomeDashboard() {
    return useQuery({
        queryKey: ["home", "dashboard"],
        queryFn: () => homeApi.dashboard(),
        refetchInterval: 60_000, // refresh every minute — live sensor data
    })
}

export function useAlerts(unreadOnly = false) {
    return useQuery({
        queryKey: ["alerts", unreadOnly],
        queryFn: () => homeApi.alerts(unreadOnly),
        refetchInterval: 30_000,
    })
}

export function useAcknowledgeAlert() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: (id: string) => homeApi.acknowledgeAlert(id),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["alerts"] }),
    })
}

export function useEnergyHistory(from: string, to: string, granularity = "daily") {
    return useQuery({
        queryKey: ["energy", from, to, granularity],
        queryFn: () => homeApi.energyHistory(from, to, granularity),
        staleTime: 60 * 60_000, // energy data changes once per day
    })
}

// ── Media ──────────────────────────────────────────────────────────────────────
import { mediaApi } from '@/api/client'
import type { JellyfinItem } from '@/types'

export function useJellyfinUser() {
    return useQuery({
        queryKey: ['jellyfin', 'user'],
        queryFn: () => mediaApi.getUser(),
        staleTime: Infinity,
        retry: false, // don't retry — if Jellyfin key is wrong, fail fast
    })
}

export function useMovies(userId: string, page = 0) {
    return useQuery({
        queryKey: ['jellyfin', 'movies', userId, page],
        queryFn: () => mediaApi.movies(userId, { StartIndex: page * 50, Limit: 50 }),
        enabled: !!userId,
        staleTime: 5 * 60_000,
    })
}

export function useSeries(userId: string, page = 0) {
    return useQuery({
        queryKey: ['jellyfin', 'series', userId, page],
        queryFn: () => mediaApi.series(userId, { StartIndex: page * 50, Limit: 50 }),
        enabled: !!userId,
        staleTime: 5 * 60_000,
    })
}

export function useTorrents() {
    return useQuery({
        queryKey: ['torrents'],
        queryFn: () => mediaApi.torrents(),
        refetchInterval: 5_000, // poll every 5s — downloads are live
        retry: false,
    })
}

export function useRemoveTorrent() {
    const qc = useQueryClient()
    return useMutation({
        mutationFn: ({ id, deleteData }: { id: number; deleteData?: boolean }) =>
            mediaApi.removeTorrent(id, deleteData),
        onSuccess: () => qc.invalidateQueries({ queryKey: ['torrents'] }),
    })
}