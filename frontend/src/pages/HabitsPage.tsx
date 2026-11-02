import { useState } from 'react'
import { Plus, Flame, Check, ChevronDown, Pencil, Trash2 } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PageHeader } from '@/components/layout/PageHeader'
import { useHabits, useCompleteHabit, useCreateHabit, useUpdateHabit, useDeleteHabit } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { Habit } from '@/types'

// ── Constants ─────────────────────────────────────────────────────────────────

// ISO weekday numbers: 1=Mon … 7=Sun
const WEEK_DAYS = [
    { id: 1, short: 'L', label: 'Lundi' },
    { id: 2, short: 'M', label: 'Mardi' },
    { id: 3, short: 'M', label: 'Mercredi' },
    { id: 4, short: 'J', label: 'Jeudi' },
    { id: 5, short: 'V', label: 'Vendredi' },
    { id: 6, short: 'S', label: 'Samedi' },
    { id: 7, short: 'D', label: 'Dimanche' },
]

const CATEGORIES = [
    { id: 'health',    label: 'Santé',     color: '#EF4444' },
    { id: 'sport',     label: 'Sport',     color: '#06B6D4' },
    { id: 'church',    label: 'Église',    color: '#F59E0B' },
    { id: 'work',      label: 'Travail',   color: '#3B82F6' },
    { id: 'learning',  label: 'Formation', color: '#8B5CF6' },
    { id: 'home',      label: 'Maison',    color: '#10B981' },
    { id: 'personal',  label: 'Personnel', color: '#EC4899' },
    { id: 'social',    label: 'Social',    color: '#84CC16' },
]

const ICONS = ['✓', '🏃', '📚', '🧘', '💪', '🎨', '🙏', '🎵', '✍️', '🌿', '💧', '🍎']

const COLORS = [
    '#1D9E75', '#3B82F6', '#EF4444', '#F59E0B',
    '#8B5CF6', '#06B6D4', '#EC4899', '#84CC16',
]

// ── Day picker ────────────────────────────────────────────────────────────────

function DayPicker({
    selected,
    onChange,
}: {
    selected: number[]
    onChange: (days: number[]) => void
}) {
    function toggle(day: number) {
        if (selected.includes(day)) {
            onChange(selected.filter(d => d !== day))
        } else {
            onChange([...selected, day].sort())
        }
    }

    return (
        <div className="flex gap-1">
            {WEEK_DAYS.map(d => (
                <button
                    key={d.id}
                    type="button"
                    onClick={() => toggle(d.id)}
                    title={d.label}
                    className={cn(
                        'flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium transition-colors',
                        selected.includes(d.id)
                            ? 'bg-primary text-primary-foreground'
                            : 'bg-muted text-muted-foreground hover:bg-muted/70 hover:text-foreground'
                    )}
                >
                    {d.short}
                </button>
            ))}
        </div>
    )
}

// ── Habit form (create + edit) ────────────────────────────────────────────────

interface HabitFormValues {
    title: string
    frequency: 'daily' | 'weekly'
    scheduledDays: number[]    // empty = every day
    targetPerWeek: number
    color: string
    icon: string
    category: string
}

function defaultValues(): HabitFormValues {
    return {
        title: '',
        frequency: 'daily',
        scheduledDays: [],
        targetPerWeek: 3,
        color: '#1D9E75',
        icon: '✓',
        category: '',
    }
}

function HabitForm({
    initial,
    onSubmit,
    onCancel,
    submitLabel = 'Ajouter',
    isPending = false,
}: {
    initial?: Partial<HabitFormValues>
    onSubmit: (values: HabitFormValues) => void
    onCancel: () => void
    submitLabel?: string
    isPending?: boolean
}) {
    const [values, setValues] = useState<HabitFormValues>({ ...defaultValues(), ...initial })
    const [expanded, setExpanded] = useState(!!initial?.title) // pre-expand on edit

    function set<K extends keyof HabitFormValues>(key: K, val: HabitFormValues[K]) {
        setValues(prev => ({ ...prev, [key]: val }))
    }

    function handleSubmit(e: React.FormEvent) {
        e.preventDefault()
        if (!values.title.trim()) return
        onSubmit(values)
    }

    const isEveryDay = values.scheduledDays.length === 0
    const daysLabel = isEveryDay
        ? 'Tous les jours'
        : values.scheduledDays.map(d => WEEK_DAYS.find(w => w.id === d)?.short).join(' ')

    return (
        <form onSubmit={handleSubmit}>
            <Card>
                <CardContent className="p-4 space-y-4">
                    {/* Title row */}
                    <div className="flex items-center gap-2">
                        {/* Icon picker */}
                        <select
                            value={values.icon}
                            onChange={e => set('icon', e.target.value)}
                            className="h-9 rounded-md border border-input bg-background px-1 text-base focus:outline-none"
                        >
                            {ICONS.map(ic => (
                                <option key={ic} value={ic}>{ic}</option>
                            ))}
                        </select>

                        <Input
                            autoFocus
                            placeholder="Nom de l'habitude…"
                            value={values.title}
                            onChange={e => set('title', e.target.value)}
                            className="border-0 shadow-none focus-visible:ring-0 p-0 text-sm flex-1"
                        />

                        <button
                            type="button"
                            onClick={() => setExpanded(!expanded)}
                            className="shrink-0 text-muted-foreground hover:text-foreground transition-colors"
                        >
                            <ChevronDown size={14} className={cn('transition-transform', expanded && 'rotate-180')} />
                        </button>
                    </div>

                    {expanded && (
                        <div className="space-y-4 border-t border-border pt-3">

                            {/* Frequency */}
                            <div className="space-y-2">
                                <Label className="text-[10px] uppercase tracking-wide text-muted-foreground">Fréquence</Label>
                                <div className="flex gap-1 rounded-lg bg-muted p-1 w-fit">
                                    {(['daily', 'weekly'] as const).map(f => (
                                        <button
                                            key={f}
                                            type="button"
                                            onClick={() => set('frequency', f)}
                                            className={cn(
                                                'rounded-md px-3 py-1 text-xs transition-colors',
                                                values.frequency === f
                                                    ? 'bg-background text-foreground shadow-sm font-medium'
                                                    : 'text-muted-foreground hover:text-foreground'
                                            )}
                                        >
                                            {f === 'daily' ? 'Quotidien' : 'Hebdomadaire'}
                                        </button>
                                    ))}
                                </div>
                            </div>

                            {/* Days of week */}
                            <div className="space-y-2">
                                <div className="flex items-center justify-between">
                                    <Label className="text-[10px] uppercase tracking-wide text-muted-foreground">
                                        Jours — <span className="normal-case font-normal">{daysLabel}</span>
                                    </Label>
                                    {!isEveryDay && (
                                        <button
                                            type="button"
                                            onClick={() => set('scheduledDays', [])}
                                            className="text-[10px] text-muted-foreground hover:text-foreground"
                                        >
                                            Tous les jours
                                        </button>
                                    )}
                                </div>
                                <DayPicker
                                    selected={values.scheduledDays}
                                    onChange={days => set('scheduledDays', days)}
                                />
                                <p className="text-[10px] text-muted-foreground">
                                    Aucun sélectionné = tous les jours
                                </p>
                            </div>

                            {/* Weekly target (only shown for weekly frequency) */}
                            {values.frequency === 'weekly' && (
                                <div className="space-y-2">
                                    <Label className="text-[10px] uppercase tracking-wide text-muted-foreground">
                                        Objectif par semaine
                                    </Label>
                                    <div className="flex items-center gap-2">
                                        <input
                                            type="range"
                                            min={1}
                                            max={7}
                                            value={values.targetPerWeek}
                                            onChange={e => set('targetPerWeek', Number(e.target.value))}
                                            className="w-32"
                                        />
                                        <span className="text-sm font-medium tabular-nums">
                                            {values.targetPerWeek}×/semaine
                                        </span>
                                    </div>
                                </div>
                            )}

                            {/* Category */}
                            <div className="space-y-2">
                                <Label className="text-[10px] uppercase tracking-wide text-muted-foreground">Catégorie</Label>
                                <div className="flex flex-wrap gap-1.5">
                                    <button
                                        type="button"
                                        onClick={() => set('category', '')}
                                        className={cn(
                                            'rounded-full px-3 py-0.5 text-xs font-medium border transition-colors',
                                            values.category === ''
                                                ? 'bg-foreground text-background border-foreground'
                                                : 'border-border text-muted-foreground hover:border-foreground hover:text-foreground'
                                        )}
                                    >
                                        Aucune
                                    </button>
                                    {CATEGORIES.map(cat => (
                                        <button
                                            key={cat.id}
                                            type="button"
                                            onClick={() => set('category', values.category === cat.id ? '' : cat.id)}
                                            className="rounded-full px-3 py-0.5 text-xs font-medium border transition-colors"
                                            style={
                                                values.category === cat.id
                                                    ? { background: cat.color, color: 'white', borderColor: cat.color }
                                                    : { borderColor: cat.color + '66', color: cat.color }
                                            }
                                        >
                                            {cat.label}
                                        </button>
                                    ))}
                                </div>
                            </div>

                            {/* Color */}
                            <div className="space-y-2">
                                <Label className="text-[10px] uppercase tracking-wide text-muted-foreground">Couleur</Label>
                                <div className="flex gap-1.5">
                                    {COLORS.map(c => (
                                        <button
                                            key={c}
                                            type="button"
                                            onClick={() => set('color', c)}
                                            className={cn(
                                                'h-6 w-6 rounded-full border-2 transition-transform',
                                                values.color === c ? 'scale-125 border-foreground' : 'border-transparent'
                                            )}
                                            style={{ background: c }}
                                        />
                                    ))}
                                </div>
                            </div>
                        </div>
                    )}

                    {/* Actions */}
                    <div className="flex justify-end gap-2 pt-1">
                        <Button type="button" size="sm" variant="ghost" onClick={onCancel}>
                            Annuler
                        </Button>
                        <Button type="submit" size="sm" disabled={!values.title.trim() || isPending}>
                            {submitLabel}
                        </Button>
                    </div>
                </CardContent>
            </Card>
        </form>
    )
}

// ── Habit card ────────────────────────────────────────────────────────────────

function HabitCard({
    habit,
    onComplete,
}: {
    habit: Habit
    onComplete: () => void
}) {
    const updateHabit = useUpdateHabit()
    const deleteHabit = useDeleteHabit()
    const [editing, setEditing] = useState(false)

    const streak = habit.current_streak
    const scheduledDays: number[] = habit.scheduled_days ?? []

    // Which days are scheduled (for the 7-dot display)
    function isDayScheduled(isoDay: number) {
        return scheduledDays.length === 0 || scheduledDays.includes(isoDay)
    }

    // Today's ISO weekday (1=Mon … 7=Sun)
    const todayIso = new Date().getDay() === 0 ? 7 : new Date().getDay()
    const scheduledToday = isDayScheduled(todayIso)

    const cat = habit.color
        ? CATEGORIES.find(c => c.color === habit.color) ?? null
        : null

    async function handleEdit(values: HabitFormValues) {
        await updateHabit.mutateAsync({
            id: habit.id,
            title: values.title,
            frequency: values.frequency,
            scheduled_days: values.scheduledDays,
            target_per_week: values.frequency === 'weekly' ? values.targetPerWeek : undefined,
            color: values.color,
            icon: values.icon,
        })
        setEditing(false)
    }

    if (editing) {
        return (
            <HabitForm
                initial={{
                    title: habit.title,
                    frequency: habit.frequency,
                    scheduledDays: habit.scheduled_days ?? [],
                    targetPerWeek: habit.target_per_week ?? 3,
                    color: habit.color,
                    icon: habit.icon,
                }}
                onSubmit={handleEdit}
                onCancel={() => setEditing(false)}
                submitLabel="Enregistrer"
                isPending={updateHabit.isPending}
            />
        )
    }

    return (
        <Card className={cn(
            'transition-colors',
            habit.completed_today && 'border-primary/30 bg-primary/5',
            !scheduledToday && 'opacity-50'
        )}>
            <CardContent className="p-4">
                <div className="flex items-center gap-4">
                    {/* Toggle button */}
                    <button
                        onClick={onComplete}
                        disabled={!scheduledToday}
                        style={{
                            borderColor: habit.completed_today ? habit.color : undefined,
                            background: habit.completed_today ? habit.color : undefined,
                        }}
                        className={cn(
                            'flex h-10 w-10 shrink-0 items-center justify-center rounded-full border-2 text-lg transition-all',
                            !habit.completed_today && scheduledToday && 'border-border hover:border-primary',
                            !scheduledToday && 'cursor-not-allowed'
                        )}
                        title={!scheduledToday ? 'Pas prévu aujourd\'hui' : undefined}
                    >
                        {habit.completed_today
                            ? <Check size={18} className="text-white" />
                            : <span>{habit.icon}</span>}
                    </button>

                    {/* Info */}
                    <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                            <p className={cn(
                                'font-medium text-sm',
                                habit.completed_today && 'text-muted-foreground'
                            )}>
                                {habit.title}
                            </p>
                            {cat && (
                                <span
                                    className="rounded-full px-2 py-0.5 text-[10px] font-medium"
                                    style={{ background: cat.color + '22', color: cat.color }}
                                >
                                    {cat.label}
                                </span>
                            )}
                            {!scheduledToday && (
                                <span className="text-[10px] text-muted-foreground">
                                    Pas aujourd'hui
                                </span>
                            )}
                        </div>

                        {/* Schedule description */}
                        <p className="text-xs text-muted-foreground mt-0.5">
                            {scheduledDays.length === 0
                                ? 'Tous les jours'
                                : scheduledDays.map(d => WEEK_DAYS.find(w => w.id === d)?.label).join(', ')}
                            {habit.frequency === 'weekly' && habit.target_per_week
                                ? ` · ${habit.target_per_week}×/semaine`
                                : ''}
                        </p>
                    </div>

                    {/* Streak */}
                    {streak > 0 && (
                        <div className="flex items-center gap-1 text-sm font-medium text-orange-500 shrink-0">
                            <Flame size={14} />
                            <span>{streak}</span>
                        </div>
                    )}

                    {/* Actions */}
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
                        <button
                            onClick={() => setEditing(true)}
                            className="text-muted-foreground hover:text-foreground transition-colors p-1"
                        >
                            <Pencil size={13} />
                        </button>
                        <button
                            onClick={() => deleteHabit.mutate(habit.id)}
                            className="text-muted-foreground hover:text-destructive transition-colors p-1"
                        >
                            <Trash2 size={13} />
                        </button>
                    </div>
                </div>

                {/* 7-day week dots */}
                <div className="mt-3 pl-14 flex gap-1">
                    {WEEK_DAYS.map((d) => {
                        const scheduled = isDayScheduled(d.id)
                        const isToday = d.id === todayIso
                        // Approximate: mark as done if within current streak and day is in the past
                        const dayIndex = d.id <= todayIso ? todayIso - d.id : 7 - (d.id - todayIso)
                        const done = scheduled && dayIndex < streak
                        const todayDone = isToday && habit.completed_today

                        return (
                            <div key={d.id} className="flex flex-col items-center gap-0.5">
                                <span className={cn(
                                    'text-[9px]',
                                    isToday ? 'text-foreground font-medium' : 'text-muted-foreground'
                                )}>
                                    {d.short}
                                </span>
                                <div
                                    className={cn(
                                        'h-2 w-2 rounded-sm transition-colors',
                                        !scheduled && 'opacity-20 bg-muted',
                                        scheduled && !done && !todayDone && 'bg-muted',
                                        (done || todayDone) && 'bg-primary',
                                        isToday && !todayDone && scheduled && 'ring-1 ring-primary ring-offset-1',
                                    )}
                                    style={(done || todayDone) ? { background: habit.color } : undefined}
                                />
                            </div>
                        )
                    })}
                </div>

                {/* Longest streak */}
                {habit.longest_streak > 1 && (
                    <p className="mt-1.5 pl-14 text-[10px] text-muted-foreground">
                        Record : {habit.longest_streak} jour{habit.longest_streak > 1 ? 's' : ''}
                    </p>
                )}
            </CardContent>
        </Card>
    )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export function HabitsPage() {
    const { data, isLoading } = useHabits()
    const completeHabit = useCompleteHabit()
    const createHabit = useCreateHabit()
    const [creating, setCreating] = useState(false)
    const [categoryFilter, setCategoryFilter] = useState<string>('all')

    const allHabits = (data?.habits ?? []).filter((h: Habit) => h.is_active)

    const habits = allHabits.filter((h: Habit) => {
        if (categoryFilter === 'all') return true
        const cat = CATEGORIES.find(c => c.color === h.color)
        return cat?.id === categoryFilter
    })

    const done = allHabits.filter((h: Habit) => h.completed_today).length

    async function handleCreate(values: HabitFormValues) {
        await createHabit.mutateAsync({
            title: values.title,
            frequency: values.frequency,
            scheduled_days: values.scheduledDays,
            target_per_week: values.frequency === 'weekly' ? values.targetPerWeek : undefined,
            color: values.color,
            icon: values.icon,
        })
        setCreating(false)
    }

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Habitudes"
                subtitle={`${done}/${allHabits.length} complétées aujourd'hui`}
                actions={
                    <Button size="sm" onClick={() => setCreating(true)}>
                        <Plus size={14} className="mr-1" /> Nouvelle habitude
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6 max-w-3xl space-y-4">
                {/* Category filter */}
                <div className="flex flex-wrap gap-1.5">
                    <button
                        onClick={() => setCategoryFilter('all')}
                        className={cn(
                            'rounded-full px-3 py-0.5 text-xs font-medium border transition-colors',
                            categoryFilter === 'all'
                                ? 'bg-foreground text-background border-foreground'
                                : 'border-border text-muted-foreground hover:text-foreground hover:border-foreground'
                        )}
                    >
                        Toutes
                    </button>
                    {CATEGORIES.map(cat => (
                        <button
                            key={cat.id}
                            onClick={() => setCategoryFilter(cat.id === categoryFilter ? 'all' : cat.id)}
                            className="rounded-full px-3 py-0.5 text-xs font-medium border transition-colors"
                            style={
                                categoryFilter === cat.id
                                    ? { background: cat.color, color: 'white', borderColor: cat.color }
                                    : { borderColor: cat.color + '66', color: cat.color }
                            }
                        >
                            {cat.label}
                        </button>
                    ))}
                </div>

                {/* Create form */}
                {creating && (
                    <HabitForm
                        onSubmit={handleCreate}
                        onCancel={() => setCreating(false)}
                        isPending={createHabit.isPending}
                    />
                )}

                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}

                {!isLoading && habits.length === 0 && (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                        <span className="text-4xl mb-3">🌱</span>
                        <p className="text-sm text-muted-foreground">Aucune habitude</p>
                        <p className="text-xs text-muted-foreground mt-1">
                            Créez votre première habitude pour commencer votre suivi
                        </p>
                    </div>
                )}

                <div className="space-y-3">
                    {habits.map((habit: Habit) => (
                        <div key={habit.id} className="group">
                            <HabitCard
                                habit={habit}
                                onComplete={() => !habit.completed_today && completeHabit.mutate({ id: habit.id })}
                            />
                        </div>
                    ))}
                </div>
            </div>
        </div>
    )
}
