import { useState } from 'react'
import { Plus, Circle, CheckCircle2, Trash2, Flag } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageHeader } from '@/components/layout/PageHeader'
import { useTasks, useCreateTask, useCompleteTask, useDeleteTask } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { Task, TaskStatus } from '@/types'

const STATUS_TABS: { id: TaskStatus | 'all'; label: string }[] = [
    { id: 'all', label: 'Toutes' },
    { id: 'todo', label: 'À faire' },
    { id: 'in_progress', label: 'En cours' },
    { id: 'done', label: 'Terminées' },
]

const PRIORITY_COLOR: Record<number, string> = {
    1: 'text-destructive',
    2: 'text-yellow-600 dark:text-yellow-400',
    3: 'text-muted-foreground',
}

const PRIORITY_LABEL: Record<number, string> = { 1: 'Haute', 2: 'Moyenne', 3: 'Basse' }

export function TasksPage() {
    const [statusFilter, setStatusFilter] = useState<TaskStatus | 'all'>('todo')
    const [newTitle, setNewTitle] = useState('')
    const [creating, setCreating] = useState(false)

    const params = statusFilter !== 'all' ? { status: statusFilter } : undefined
    const { data, isLoading } = useTasks(params)
    const createTask = useCreateTask()
    const completeTask = useCompleteTask()
    const deleteTask = useDeleteTask()

    const tasks = data?.tasks ?? []

    async function handleCreate(e: React.FormEvent) {
        e.preventDefault()
        if (!newTitle.trim()) return
        await createTask.mutateAsync({ title: newTitle.trim(), priority: 2 })
        setNewTitle('')
        setCreating(false)
    }

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Tâches"
                subtitle={`${data?.count ?? 0} tâche${(data?.count ?? 0) !== 1 ? 's' : ''}`}
                actions={
                    <Button size="sm" onClick={() => setCreating(true)}>
                        <Plus size={14} className="mr-1" /> Nouvelle tâche
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6">
                {/* Status filter tabs */}
                <div className="mb-4 flex gap-1 rounded-lg bg-muted p-1 w-fit">
                    {STATUS_TABS.map(tab => (
                        <button
                            key={tab.id}
                            onClick={() => setStatusFilter(tab.id)}
                            className={cn(
                                'rounded-md px-3 py-1.5 text-sm transition-colors',
                                statusFilter === tab.id
                                    ? 'bg-background text-foreground shadow-sm font-medium'
                                    : 'text-muted-foreground hover:text-foreground'
                            )}
                        >
                            {tab.label}
                        </button>
                    ))}
                </div>

                {/* Inline create */}
                {creating && (
                    <form onSubmit={handleCreate} className="mb-3">
                        <Card>
                            <CardContent className="flex items-center gap-2 p-3">
                                <Circle size={18} className="shrink-0 text-muted-foreground" />
                                <Input
                                    autoFocus
                                    placeholder="Titre de la tâche…"
                                    value={newTitle}
                                    onChange={e => setNewTitle(e.target.value)}
                                    className="border-0 shadow-none focus-visible:ring-0 p-0 text-sm"
                                />
                                <Button type="submit" size="sm" disabled={!newTitle.trim() || createTask.isPending}>
                                    Ajouter
                                </Button>
                                <Button type="button" size="sm" variant="ghost" onClick={() => { setCreating(false); setNewTitle('') }}>
                                    Annuler
                                </Button>
                            </CardContent>
                        </Card>
                    </form>
                )}

                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}

                {!isLoading && tasks.length === 0 && (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                        <CheckCircle2 size={40} className="mb-3 text-muted-foreground/30" />
                        <p className="text-sm text-muted-foreground">Aucune tâche ici</p>
                    </div>
                )}

                <div className="space-y-1">
                    {tasks.map((task: Task) => (
                        <div
                            key={task.id}
                            className="group flex items-center gap-3 rounded-lg border border-transparent px-3 py-2.5 hover:border-border hover:bg-muted/30 transition-all"
                        >
                            <button
                                onClick={() => completeTask.mutate(task.id)}
                                disabled={task.status === 'done'}
                                className="shrink-0 text-muted-foreground hover:text-primary disabled:opacity-50 transition-colors"
                            >
                                {task.status === 'done'
                                    ? <CheckCircle2 size={18} className="text-primary" />
                                    : <Circle size={18} />}
                            </button>
                            <div className="flex-1 min-w-0">
                                <p className={cn('truncate text-sm', task.status === 'done' && 'text-muted-foreground line-through')}>
                                    {task.title}
                                </p>
                                {task.due_date && (
                                    <p className="text-xs text-muted-foreground">
                                        Échéance : {new Date(task.due_date).toLocaleDateString('fr-FR')}
                                    </p>
                                )}
                            </div>
                            {task.tags?.length > 0 && (
                                <div className="hidden items-center gap-1 sm:flex">
                                    {task.tags.slice(0, 2).map(tag => (
                                        <span
                                            key={tag.id}
                                            className="rounded-full px-2 py-0.5 text-[10px] font-medium"
                                            style={{ background: tag.color + '22', color: tag.color }}
                                        >
                                            {tag.name}
                                        </span>
                                    ))}
                                </div>
                            )}
                            <Flag size={13} className={cn('shrink-0', PRIORITY_COLOR[task.priority])} />
                            <button
                                onClick={() => deleteTask.mutate(task.id)}
                                className="shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive transition-all"
                            >
                                <Trash2 size={14} />
                            </button>
                        </div>
                    ))}
                </div>
            </div>
        </div>
    )
}