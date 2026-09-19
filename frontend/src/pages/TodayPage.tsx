import { CheckCircle2, Circle, Flame, Zap, Wallet, Thermometer, AlertTriangle, TrendingDown, TrendingUp } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { StatCard } from '@/components/layout/StatCard'
import { useNav } from '@/store/navigation'
import { useTodayTasks, useHabits, useCurrentBudget, useHomeDashboard, useCompleteTask, useCompleteHabit } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { Task, Habit } from '@/types'

function fmt(cents: number) {
    return (cents / 100).toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}

export function TodayPage() {
    const { setActive } = useNav()
    const { data: tasksData } = useTodayTasks()
    const { data: habitsData } = useHabits()
    const { data: budgetData } = useCurrentBudget()
    const { data: homeData } = useHomeDashboard()
    const completeTask = useCompleteTask()
    const completeHabit = useCompleteHabit()

    const tasks = tasksData?.tasks ?? []
    const habits = (habitsData?.habits ?? []).filter((h: Habit) => h.is_active)
    const doneHabits = habits.filter((h: Habit) => h.completed_today).length
    const interior = homeData?.latest_temperatures?.find(t => t.source === 'interior')

    const now = new Date()
    const dateStr = now.toLocaleDateString('fr-FR', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })

    return (
        <div className="flex-1 overflow-y-auto">
            {/* Header */}
            <div className="border-b border-border px-6 py-5">
                <h1 className="text-xl font-semibold capitalize">{dateStr}</h1>
                <p className="mt-0.5 text-sm text-muted-foreground">Votre journée en un coup d'œil.</p>
            </div>

            <div className="p-6 space-y-6 max-w-6xl">
                {/* KPI row */}
                <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                    <StatCard
                        label="Tâches restantes"
                        value={tasks.filter(t => t.status !== 'done').length}
                        sub="pour aujourd'hui"
                        icon={<CheckCircle2 size={15} />}
                    />
                    <StatCard
                        label="Habitudes"
                        value={`${doneHabits}/${habits.length}`}
                        sub="complétées"
                        icon={<Flame size={15} />}
                    />
                    <StatCard
                        label="Conso. hier"
                        value={homeData?.today_kwh != null ? `${homeData.today_kwh.toFixed(1)} kWh` : '—'}
                        icon={<Zap size={15} />}
                    />
                    <StatCard
                        label="Budget restant"
                        value={budgetData ? fmt(budgetData.summary.remaining_cents) : '—'}
                        sub={budgetData ? `${Math.round((budgetData.summary.total_spent_euros / budgetData.summary.total_budgeted_euros) * 100)}% utilisé` : undefined}
                        icon={<Wallet size={15} />}
                    />
                </div>

                {/* Two-column grid */}
                <div className="grid gap-4 lg:grid-cols-2">
                    {/* Left column */}
                    <div className="space-y-4">
                        {/* Tasks */}
                        <Card>
                            <CardHeader className="flex flex-row items-center justify-between pb-3">
                                <CardTitle className="text-sm font-medium">Tâches du jour</CardTitle>
                                <Button variant="ghost" size="sm" className="text-xs text-muted-foreground" onClick={() => setActive('planning.tasks')}>
                                    Tout voir →
                                </Button>
                            </CardHeader>
                            <CardContent className="space-y-1">
                                {tasks.length === 0 && (
                                    <p className="py-4 text-center text-sm text-muted-foreground">Aucune tâche pour aujourd'hui 🎉</p>
                                )}
                                {tasks.slice(0, 5).map((task: Task) => (
                                    <div key={task.id} className="flex items-center gap-3 rounded-md px-1 py-1.5 hover:bg-muted/50">
                                        <button
                                            onClick={() => completeTask.mutate(task.id)}
                                            disabled={task.status === 'done'}
                                            className="shrink-0 text-muted-foreground hover:text-primary disabled:opacity-50"
                                        >
                                            {task.status === 'done'
                                                ? <CheckCircle2 size={18} className="text-primary" />
                                                : <Circle size={18} />}
                                        </button>
                                        <span className={cn('flex-1 text-sm', task.status === 'done' && 'text-muted-foreground line-through')}>
                                            {task.title}
                                        </span>
                                    </div>
                                ))}
                            </CardContent>
                        </Card>

                        {/* Habits */}
                        <Card>
                            <CardHeader className="flex flex-row items-center justify-between pb-3">
                                <CardTitle className="text-sm font-medium">Habitudes</CardTitle>
                                <Button variant="ghost" size="sm" className="text-xs text-muted-foreground" onClick={() => setActive('planning.habits')}>
                                    Tout voir →
                                </Button>
                            </CardHeader>
                            <CardContent className="space-y-2">
                                {habits.length === 0 && (
                                    <p className="py-4 text-center text-sm text-muted-foreground">Aucune habitude active</p>
                                )}
                                {habits.map((habit: Habit) => (
                                    <div key={habit.id} className="flex items-center gap-3">
                                        <button
                                            onClick={() => !habit.completed_today && completeHabit.mutate({ id: habit.id })}
                                            className={cn(
                                                'flex h-7 w-7 shrink-0 items-center justify-center rounded-full border-2 text-sm transition-colors',
                                                habit.completed_today
                                                    ? 'border-primary bg-primary text-primary-foreground'
                                                    : 'border-border hover:border-primary'
                                            )}
                                        >
                                            {habit.completed_today ? '✓' : habit.icon}
                                        </button>
                                        <span className={cn('flex-1 text-sm', habit.completed_today && 'text-muted-foreground line-through')}>
                                            {habit.title}
                                        </span>
                                        {habit.current_streak > 0 && (
                                            <span className="flex items-center gap-1 text-xs text-orange-500">
                                                <Flame size={12} />{habit.current_streak}j
                                            </span>
                                        )}
                                    </div>
                                ))}
                            </CardContent>
                        </Card>
                    </div>

                    {/* Right column */}
                    <div className="space-y-4">
                        {/* Home snapshot */}
                        <Card>
                            <CardHeader className="flex flex-row items-center justify-between pb-3">
                                <CardTitle className="text-sm font-medium">Maison</CardTitle>
                                <Button variant="ghost" size="sm" className="text-xs text-muted-foreground" onClick={() => setActive('home.dashboard')}>
                                    Détail →
                                </Button>
                            </CardHeader>
                            <CardContent className="space-y-3">
                                <div className="grid grid-cols-2 gap-2">
                                    {homeData?.latest_temperatures?.slice(0, 2).map(t => (
                                        <div key={t.id} className="rounded-lg bg-muted/50 p-3">
                                            <p className="text-xs text-muted-foreground capitalize">{t.source === 'interior' ? 'Intérieur' : 'Extérieur'}</p>
                                            <p className="mt-0.5 text-xl font-semibold tabular-nums">{t.celsius.toFixed(1)}°C</p>
                                            <p className="text-xs text-muted-foreground truncate">{t.room_name}</p>
                                        </div>
                                    ))}
                                    {!homeData && (
                                        <div className="col-span-2 flex items-center gap-2 rounded-lg bg-muted/50 p-3">
                                            <Thermometer size={16} className="text-muted-foreground" />
                                            <span className="text-sm text-muted-foreground">En attente des capteurs…</span>
                                        </div>
                                    )}
                                </div>
                                {homeData?.today_kwh != null && (
                                    <div className="flex items-center gap-2 rounded-lg bg-muted/50 px-3 py-2">
                                        <Zap size={14} className="text-yellow-500" />
                                        <span className="flex-1 text-sm text-muted-foreground">Conso. hier</span>
                                        <span className="text-sm font-medium tabular-nums">{homeData.today_kwh.toFixed(2)} kWh</span>
                                    </div>
                                )}
                                {(homeData?.unread_alerts ?? 0) > 0 && (
                                    <button
                                        onClick={() => setActive('home.alerts')}
                                        className="flex w-full items-center gap-2 rounded-lg bg-destructive/10 px-3 py-2 text-destructive hover:bg-destructive/20 transition-colors"
                                    >
                                        <AlertTriangle size={14} />
                                        <span className="flex-1 text-sm">{homeData!.unread_alerts} alerte{homeData!.unread_alerts > 1 ? 's' : ''} non lue{homeData!.unread_alerts > 1 ? 's' : ''}</span>
                                        <span className="text-xs">Voir →</span>
                                    </button>
                                )}
                            </CardContent>
                        </Card>

                        {/* Budget snapshot */}
                        <Card>
                            <CardHeader className="flex flex-row items-center justify-between pb-3">
                                <CardTitle className="text-sm font-medium">
                                    Budget {now.toLocaleDateString('fr-FR', { month: 'long' })}
                                </CardTitle>
                                <Button variant="ghost" size="sm" className="text-xs text-muted-foreground" onClick={() => setActive('budget.overview')}>
                                    Détail →
                                </Button>
                            </CardHeader>
                            <CardContent>
                                {!budgetData ? (
                                    <p className="text-sm text-muted-foreground">Chargement…</p>
                                ) : (
                                    <>
                                        <div className="flex items-baseline justify-between">
                                            <span className="text-2xl font-semibold tabular-nums">{fmt(budgetData.summary.total_spent_cents)}</span>
                                            <span className="text-sm text-muted-foreground">/ {fmt(budgetData.summary.total_budgeted_cents)}</span>
                                        </div>
                                        <div className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-muted">
                                            <div
                                                className={cn('h-full rounded-full transition-all', budgetData.summary.total_spent_euros > budgetData.summary.total_budgeted_euros ? 'bg-destructive' : 'bg-primary')}
                                                style={{ width: `${Math.min((budgetData.summary.total_spent_euros / budgetData.summary.total_budgeted_euros) * 100, 100)}%` }}
                                            />
                                        </div>
                                        <p className="mt-1.5 text-xs text-muted-foreground">
                                            Reste {fmt(budgetData.summary.remaining_cents)} ce mois
                                        </p>
                                        <div className="mt-3 space-y-1.5">
                                            {budgetData.envelopes.slice(0, 3).map(env => (
                                                <div key={env.id} className="flex items-center gap-2 text-xs">
                                                    <span>{env.icon}</span>
                                                    <span className="flex-1 truncate text-muted-foreground">{env.name}</span>
                                                    <div className="w-16 h-1 overflow-hidden rounded-full bg-muted">
                                                        <div
                                                            className={cn('h-full rounded-full', env.percent_used > 100 ? 'bg-destructive' : 'bg-primary/60')}
                                                            style={{ width: `${Math.min(env.percent_used, 100)}%` }}
                                                        />
                                                    </div>
                                                    <span className="w-8 text-right tabular-nums">{Math.round(env.percent_used)}%</span>
                                                </div>
                                            ))}
                                        </div>
                                    </>
                                )}
                            </CardContent>
                        </Card>
                    </div>
                </div>
            </div>
        </div>
    )
}