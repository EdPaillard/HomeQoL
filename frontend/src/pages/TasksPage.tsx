import { useState } from 'react'
import { Plus, Circle, CheckCircle2, Trash2, Flag, Clock, ChevronDown, Calendar } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PageHeader } from '@/components/layout/PageHeader'
import { useTasks, useCreateTask, useUpdateTask, useDeleteTask } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { Task, TaskStatus } from '@/types'

// ── Constants ─────────────────────────────────────────────────────────────────

const STATUS_TABS: { id: TaskStatus | 'all'; label: string }[] = [
    { id: 'all',         label: 'Toutes' },
    { id: 'todo',        label: 'À faire' },
    { id: 'in_progress', label: 'En cours' },
    { id: 'done',        label: 'Terminées' },
]

const PRIORITY_COLOR: Record<number, string> = {
    1: 'text-destructive',
    2: 'text-yellow-600 dark:text-yellow-400',
    3: 'text-muted-foreground',
}

const PRIORITY_LABELS: Record<number, string> = {
    1: 'Haute', 2: 'Moyenne', 3: 'Basse',
}

// Status cycle: todo → in_progress → done → todo
const NEXT_STATUS: Record<TaskStatus, TaskStatus> = {
    todo:        'in_progress',
    in_progress: 'done',
    done:        'todo',
    cancelled:   'todo',
}

const STATUS_ICON = {
    todo:        <Circle size={18} />,
    in_progress: <Clock size={18} className="text-yellow-500" />,
    done:        <CheckCircle2 size={18} className="text-primary" />,
    cancelled:   <Circle size={18} className="text-muted-foreground" />,
}

// Predefined categories — you can extend this list
const CATEGORIES = [
    { id: 'work',      label: 'Travail',   color: '#3B82F6' },
    { id: 'home',      label: 'Maison',    color: '#10B981' },
    { id: 'health',    label: 'Santé',     color: '#EF4444' },
    { id: 'personal',  label: 'Personnel', color: '#8B5CF6' },
    { id: 'church',    label: 'Église',    color: '#F59E0B' },
    { id: 'sport',     label: 'Sport',     color: '#06B6D4' },
    { id: 'finance',   label: 'Finance',   color: '#84CC16' },
    { id: 'learning',  label: 'Formation', color: '#EC4899' },
]

// ── Create task form ──────────────────────────────────────────────────────────

function CreateTaskForm({ onClose }: { onClose: () => void }) {
    const createTask = useCreateTask()
    const [title, setTitle]       = useState('')
    const [priority, setPriority] = useState(2)
    const [dueDate, setDueDate]   = useState('')
    const [category, setCategory] = useState('')
    const [expanded, setExpanded] = useState(false)

    async function handleSubmit(e: React.FormEvent) {
        e.preventDefault()
        if (!title.trim()) return
        const tagIds = category ? [category] : []
        await createTask.mutateAsync({
            title: title.trim(),
            priority,
            due_date: dueDate || undefined,
            tag_ids: tagIds,
        })
        onClose()
    }

    return (
        <Card className="mb-3">
            <CardContent className="p-3">
                <form onSubmit={handleSubmit} className="space-y-3">
                    <div className="flex items-center gap-2">
                        <Circle size={18} className="shrink-0 text-muted-foreground" />
                        <Input
                            autoFocus
                            placeholder="Titre de la tâche…"
                            value={title}
                            onChange={e => setTitle(e.target.value)}
                            className="border-0 shadow-none focus-visible:ring-0 p-0 text-sm"
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
                        <div className="grid grid-cols-3 gap-2 pl-6">
                            {/* Priority */}
                            <div className="space-y-1">
                                <Label className="text-[10px] uppercase tracking-wide">Priorité</Label>
                                <select
                                    value={priority}
                                    onChange={e => setPriority(Number(e.target.value))}
                                    className="w-full rounded-md border border-input bg-background px-2 py-1 text-xs focus:outline-none"
                                >
                                    <option value={1}>🔴 Haute</option>
                                    <option value={2}>🟡 Moyenne</option>
                                    <option value={3}>⚪ Basse</option>
                                </select>
                            </div>

                            {/* Due date */}
                            <div className="space-y-1">
                                <Label className="text-[10px] uppercase tracking-wide">Échéance</Label>
                                <Input
                                    type="date"
                                    value={dueDate}
                                    onChange={e => setDueDate(e.target.value)}
                                    className="h-7 text-xs px-2"
                                />
                            </div>

                            {/* Category */}
                            <div className="space-y-1">
                                <Label className="text-[10px] uppercase tracking-wide">Catégorie</Label>
                                <select
                                    value={category}
                                    onChange={e => setCategory(e.target.value)}
                                    className="w-full rounded-md border border-input bg-background px-2 py-1 text-xs focus:outline-none"
                                >
                                    <option value="">Aucune</option>
                                    {CATEGORIES.map(c => (
                                        <option key={c.id} value={c.id}>{c.label}</option>
                                    ))}
                                </select>
                            </div>
                        </div>
                    )}

                    <div className="flex justify-end gap-2">
                        <Button type="button" size="sm" variant="ghost" onClick={onClose}>Annuler</Button>
                        <Button type="submit" size="sm" disabled={!title.trim() || createTask.isPending}>
                            Ajouter
                        </Button>
                    </div>
                </form>
            </CardContent>
        </Card>
    )
}

// ── Task row ──────────────────────────────────────────────────────────────────

function TaskRow({ task }: { task: Task }) {
    const updateTask = useUpdateTask()
    const deleteTask = useDeleteTask()
    const [editing, setEditing] = useState(false)
    const [editTitle, setEditTitle] = useState(task.title)

    function cycleStatus() {
        const next = NEXT_STATUS[task.status]
        updateTask.mutate({ id: task.id, status: next })
    }

    function saveTitle() {
        if (editTitle.trim() && editTitle !== task.title) {
            updateTask.mutate({ id: task.id, title: editTitle.trim() })
        }
        setEditing(false)
    }

    const isDone = task.status === 'done'
    const cat = task.tags?.[0]
        ? CATEGORIES.find(c => c.id === task.tags[0].id) ?? null
        : null

    return (
        <div className="group flex items-center gap-3 rounded-lg border border-transparent px-3 py-2.5 hover:border-border hover:bg-muted/30 transition-all">
            {/* Status cycle button */}
            <button
                onClick={cycleStatus}
                disabled={updateTask.isPending}
                className="shrink-0 text-muted-foreground hover:text-primary transition-colors"
                title={`Passer à : ${PRIORITY_LABELS[task.priority]} → ${NEXT_STATUS[task.status]}`}
            >
                {STATUS_ICON[task.status]}
            </button>

            {/* Title — click to edit */}
            <div className="flex-1 min-w-0">
                {editing ? (
                    <Input
                        autoFocus
                        value={editTitle}
                        onChange={e => setEditTitle(e.target.value)}
                        onBlur={saveTitle}
                        onKeyDown={e => { if (e.key === 'Enter') saveTitle(); if (e.key === 'Escape') setEditing(false) }}
                        className="border-0 shadow-none focus-visible:ring-0 p-0 text-sm h-auto"
                    />
                ) : (
                    <p
                        onClick={() => setEditing(true)}
                        className={cn(
                            'truncate text-sm cursor-text',
                            isDone && 'text-muted-foreground line-through'
                        )}
                    >
                        {task.title}
                    </p>
                )}
                {task.due_date && (
                    <p className={cn(
                        'text-xs flex items-center gap-1',
                        new Date(task.due_date) < new Date() && !isDone
                            ? 'text-destructive'
                            : 'text-muted-foreground'
                    )}>
                        <Calendar size={10} />
                        {new Date(task.due_date).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })}
                    </p>
                )}
            </div>

            {/* Category badge */}
            {cat && (
                <span
                    className="hidden sm:inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-medium shrink-0"
                    style={{ background: cat.color + '22', color: cat.color }}
                >
                    {cat.label}
                </span>
            )}

            {/* Status label (visible on hover) */}
            <span className={cn(
                'hidden sm:inline text-[10px] font-medium opacity-0 group-hover:opacity-100 transition-opacity shrink-0',
                task.status === 'in_progress' ? 'text-yellow-500' : 'text-muted-foreground'
            )}>
                {task.status === 'in_progress' ? 'En cours →' : task.status === 'todo' ? 'Démarrer →' : ''}
            </span>

            {/* Priority flag */}
            <Flag size={13} className={cn('shrink-0', PRIORITY_COLOR[task.priority])} />

            {/* Delete */}
            <button
                onClick={() => deleteTask.mutate(task.id)}
                className="shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive transition-all"
            >
                <Trash2 size={14} />
            </button>
        </div>
    )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export function TasksPage() {
    const [statusFilter, setStatusFilter] = useState<TaskStatus | 'all'>('todo')
    const [categoryFilter, setCategoryFilter] = useState<string>('all')
    const [creating, setCreating] = useState(false)

    const params: Record<string, string> = {}
    if (statusFilter !== 'all') params.status = statusFilter

    const { data, isLoading } = useTasks(params)

    // Client-side category filter
    const tasks = (data?.tasks ?? []).filter((t: Task) => {
        if (categoryFilter === 'all') return true
        return t.tags?.some(tag => tag.id === categoryFilter)
    })

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Tâches"
                subtitle={`${tasks.length} tâche${tasks.length !== 1 ? 's' : ''}`}
                actions={
                    <Button size="sm" onClick={() => setCreating(true)}>
                        <Plus size={14} className="mr-1" /> Nouvelle tâche
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6">
                {/* Status tabs */}
                <div className="mb-3 flex gap-1 rounded-lg bg-muted p-1 w-fit">
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
                            {tab.id === 'in_progress' ? '⏳ En cours' : tab.label}
                        </button>
                    ))}
                </div>

                {/* Category filter pills */}
                <div className="mb-4 flex flex-wrap gap-1.5">
                    <button
                        onClick={() => setCategoryFilter('all')}
                        className={cn(
                            'rounded-full px-3 py-0.5 text-xs font-medium transition-colors border',
                            categoryFilter === 'all'
                                ? 'bg-foreground text-background border-foreground'
                                : 'border-border text-muted-foreground hover:border-foreground hover:text-foreground'
                        )}
                    >
                        Toutes
                    </button>
                    {CATEGORIES.map(cat => (
                        <button
                            key={cat.id}
                            onClick={() => setCategoryFilter(cat.id === categoryFilter ? 'all' : cat.id)}
                            className={cn(
                                'rounded-full px-3 py-0.5 text-xs font-medium transition-colors border'
                            )}
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

                {/* Inline create */}
                {creating && <CreateTaskForm onClose={() => setCreating(false)} />}

                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}

                {!isLoading && tasks.length === 0 && (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                        <CheckCircle2 size={40} className="mb-3 text-muted-foreground/30" />
                        <p className="text-sm text-muted-foreground">Aucune tâche ici</p>
                        {statusFilter === 'in_progress' && (
                            <p className="mt-1 text-xs text-muted-foreground">
                                Cliquez sur ○ d'une tâche "À faire" pour la démarrer
                            </p>
                        )}
                    </div>
                )}

                <div className="space-y-1">
                    {tasks.map((task: Task) => (
                        <TaskRow key={task.id} task={task} />
                    ))}
                </div>
            </div>
        </div>
    )
}
