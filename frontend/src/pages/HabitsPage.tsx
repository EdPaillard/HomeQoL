import { useState } from 'react'
import { Plus, Flame, Check } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageHeader } from '@/components/layout/PageHeader'
import { useHabits, useCompleteHabit, useCreateHabit } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { Habit } from '@/types'

const WEEK_DAYS = ['L', 'M', 'M', 'J', 'V', 'S', 'D']

export function HabitsPage() {
    const { data, isLoading } = useHabits()
    const completeHabit = useCompleteHabit()
    const createHabit = useCreateHabit()
    const [creating, setCreating] = useState(false)
    const [newTitle, setNewTitle] = useState('')

    const habits = (data?.habits ?? []).filter((h: Habit) => h.is_active)
    const done = habits.filter((h: Habit) => h.completed_today).length

    async function handleCreate(e: React.FormEvent) {
        e.preventDefault()
        if (!newTitle.trim()) return
        await createHabit.mutateAsync({ title: newTitle.trim() })
        setNewTitle('')
        setCreating(false)
    }

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Habitudes"
                subtitle={`${done}/${habits.length} complétées aujourd'hui`}
                actions={
                    <Button size="sm" onClick={() => setCreating(true)}>
                        <Plus size={14} className="mr-1" /> Nouvelle habitude
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6 max-w-3xl">
                {creating && (
                    <form onSubmit={handleCreate} className="mb-4">
                        <Card>
                            <CardContent className="flex items-center gap-2 p-3">
                                <Input
                                    autoFocus
                                    placeholder="Nom de l'habitude…"
                                    value={newTitle}
                                    onChange={e => setNewTitle(e.target.value)}
                                    className="border-0 shadow-none focus-visible:ring-0 p-0 text-sm"
                                />
                                <Button type="submit" size="sm" disabled={!newTitle.trim()}>Ajouter</Button>
                                <Button type="button" size="sm" variant="ghost" onClick={() => { setCreating(false); setNewTitle('') }}>Annuler</Button>
                            </CardContent>
                        </Card>
                    </form>
                )}

                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}

                <div className="space-y-3">
                    {habits.map((habit: Habit) => (
                        <HabitCard
                            key={habit.id}
                            habit={habit}
                            onComplete={() => !habit.completed_today && completeHabit.mutate({ id: habit.id })}
                        />
                    ))}
                </div>
            </div>
        </div>
    )
}

function HabitCard({ habit, onComplete }: { habit: Habit; onComplete: () => void }) {
    // Build a 7-slot week view using current_streak as approximation
    const streak = habit.current_streak
    const dots = Array.from({ length: 7 }, (_, i) => i < Math.min(streak, 7))

    return (
        <Card className={cn(habit.completed_today && 'border-primary/30 bg-primary/5')}>
            <CardContent className="p-4">
                <div className="flex items-center gap-4">
                    {/* Toggle button */}
                    <button
                        onClick={onComplete}
                        style={{ borderColor: habit.completed_today ? habit.color : undefined, background: habit.completed_today ? habit.color : undefined }}
                        className={cn(
                            'flex h-10 w-10 shrink-0 items-center justify-center rounded-full border-2 text-lg transition-all',
                            !habit.completed_today && 'border-border hover:border-primary'
                        )}
                    >
                        {habit.completed_today
                            ? <Check size={18} className="text-white" />
                            : <span>{habit.icon}</span>}
                    </button>

                    {/* Info */}
                    <div className="flex-1 min-w-0">
                        <p className={cn('font-medium text-sm', habit.completed_today && 'text-muted-foreground')}>
                            {habit.title}
                        </p>
                        <p className="text-xs text-muted-foreground capitalize">
                            {habit.frequency === 'daily' ? 'Quotidien' : 'Hebdomadaire'}
                        </p>
                    </div>

                    {/* Streak */}
                    {streak > 0 && (
                        <div className="flex items-center gap-1 text-sm font-medium text-orange-500">
                            <Flame size={14} />
                            <span>{streak}</span>
                        </div>
                    )}

                    {/* 7-day dots */}
                    <div className="hidden sm:flex flex-col items-end gap-1">
                        <div className="flex gap-0.5">
                            {WEEK_DAYS.map((d, i) => (
                                <div key={i} className="flex flex-col items-center gap-0.5">
                                    <span className="text-[9px] text-muted-foreground">{d}</span>
                                    <div
                                        className={cn('h-2 w-2 rounded-sm', dots[i] ? 'bg-primary' : 'bg-muted')}
                                        style={dots[i] ? { background: habit.color } : undefined}
                                    />
                                </div>
                            ))}
                        </div>
                    </div>
                </div>

                {/* Longest streak */}
                {habit.longest_streak > 0 && (
                    <p className="mt-2 text-xs text-muted-foreground pl-14">
                        Record : {habit.longest_streak} jour{habit.longest_streak > 1 ? 's' : ''}
                    </p>
                )}
            </CardContent>
        </Card>
    )
}