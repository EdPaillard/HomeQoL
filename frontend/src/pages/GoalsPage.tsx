import { useState } from 'react'
import { Plus, Target, ChevronRight, CheckCircle2 } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageHeader } from '@/components/layout/PageHeader'
import { useGoals, useCreateGoal, useUpdateGoal } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { Goal } from '@/types'

const STATUS_TABS = [
    { id: 'active', label: 'Actifs' },
    { id: 'completed', label: 'Terminés' },
]

export function GoalsPage() {
    const { data, isLoading } = useGoals()
    const createGoal = useCreateGoal()
    const updateGoal = useUpdateGoal()
    const [creating, setCreating] = useState(false)
    const [newTitle, setNewTitle] = useState('')
    const [filter, setFilter] = useState('active')

    const goals = (data?.goals ?? []).filter((g: Goal) => g.status === filter)

    async function handleCreate(e: React.FormEvent) {
        e.preventDefault()
        if (!newTitle.trim()) return
        await createGoal.mutateAsync({ title: newTitle.trim() })
        setNewTitle('')
        setCreating(false)
    }

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Objectifs"
                subtitle="Vos objectifs à moyen et long terme"
                actions={
                    <Button size="sm" onClick={() => setCreating(true)}>
                        <Plus size={14} className="mr-1" /> Nouvel objectif
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6 max-w-3xl">
                {/* Filter tabs */}
                <div className="mb-4 flex gap-1 rounded-lg bg-muted p-1 w-fit">
                    {STATUS_TABS.map(tab => (
                        <button
                            key={tab.id}
                            onClick={() => setFilter(tab.id)}
                            className={cn(
                                'rounded-md px-3 py-1.5 text-sm transition-colors',
                                filter === tab.id
                                    ? 'bg-background text-foreground shadow-sm font-medium'
                                    : 'text-muted-foreground hover:text-foreground'
                            )}
                        >
                            {tab.label}
                        </button>
                    ))}
                </div>

                {creating && (
                    <form onSubmit={handleCreate} className="mb-4">
                        <Card>
                            <CardContent className="flex items-center gap-2 p-3">
                                <Input
                                    autoFocus
                                    placeholder="Titre de l'objectif…"
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

                {!isLoading && goals.length === 0 && (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                        <Target size={40} className="mb-3 text-muted-foreground/30" />
                        <p className="text-sm text-muted-foreground">Aucun objectif {filter === 'active' ? 'actif' : 'terminé'}</p>
                    </div>
                )}

                <div className="space-y-3">
                    {goals.map((goal: Goal) => (
                        <GoalCard
                            key={goal.id}
                            goal={goal}
                            onComplete={() => updateGoal.mutate({ id: goal.id, status: 'completed' })}
                        />
                    ))}
                </div>
            </div>
        </div>
    )
}

function GoalCard({ goal, onComplete }: { goal: Goal; onComplete: () => void }) {
    const [expanded, setExpanded] = useState(false)

    return (
        <Card>
            <CardContent className="p-4">
                <div className="flex items-start gap-3">
                    <button
                        onClick={onComplete}
                        disabled={goal.status === 'completed'}
                        className="mt-0.5 shrink-0 text-muted-foreground hover:text-primary disabled:opacity-50 transition-colors"
                    >
                        {goal.status === 'completed'
                            ? <CheckCircle2 size={18} className="text-primary" />
                            : <Target size={18} />}
                    </button>

                    <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                            <p className={cn('font-medium text-sm', goal.status === 'completed' && 'text-muted-foreground line-through')}>
                                {goal.title}
                            </p>
                            {goal.target_date && (
                                <span className="text-[10px] text-muted-foreground border border-border rounded px-1.5 py-0.5">
                                    {new Date(goal.target_date).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })}
                                </span>
                            )}
                        </div>

                        {goal.description && (
                            <p className="mt-0.5 text-xs text-muted-foreground">{goal.description}</p>
                        )}

                        {/* Progress bar */}
                        {goal.task_count > 0 && (
                            <div className="mt-2">
                                <div className="flex justify-between text-[10px] text-muted-foreground mb-1">
                                    <span>{goal.done_count}/{goal.task_count} tâches</span>
                                    <span>{Math.round(goal.progress_pct)}%</span>
                                </div>
                                <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                                    <div
                                        className="h-full rounded-full bg-primary transition-all"
                                        style={{ width: `${goal.progress_pct}%` }}
                                    />
                                </div>
                            </div>
                        )}

                        {/* Milestones */}
                        {goal.milestones?.length > 0 && (
                            <div className="mt-2">
                                <button
                                    onClick={() => setExpanded(!expanded)}
                                    className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                                >
                                    <ChevronRight size={12} className={cn('transition-transform', expanded && 'rotate-90')} />
                                    {goal.milestones.length} jalons
                                </button>
                                {expanded && (
                                    <div className="mt-2 space-y-1 pl-4 border-l-2 border-border">
                                        {goal.milestones.map(m => (
                                            <div key={m.id} className="flex items-center gap-2 text-xs text-muted-foreground">
                                                {m.status === 'completed'
                                                    ? <CheckCircle2 size={12} className="text-primary" />
                                                    : <Target size={12} />}
                                                <span className={cn(m.status === 'completed' && 'line-through')}>{m.title}</span>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>
            </CardContent>
        </Card>
    )
}