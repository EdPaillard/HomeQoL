import { useState } from 'react'
import { Plus, ArrowDownLeft, ArrowUpRight, Search, Filter } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { PageHeader } from '@/components/layout/PageHeader'
import { useTransactions, useCreateTransaction, useCurrentBudget } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { Transaction, Envelope } from '@/types'

function fmt(cents: number) {
    return (cents / 100).toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}

function fmtDate(s: string) {
    return new Date(s).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })
}

// ── Add transaction form ──────────────────────────────────────────────────────

function AddTransactionPanel({
    envelopes,
    onClose,
}: {
    envelopes: Envelope[]
    onClose: () => void
}) {
    const create = useCreateTransaction()
    const [label, setLabel] = useState('')
    const [amount, setAmount] = useState('')
    const [date, setDate] = useState(new Date().toISOString().split('T')[0])
    const [envelopeId, setEnvelopeId] = useState('')
    const [type, setType] = useState<'expense' | 'income'>('expense')

    async function handleSubmit(e: React.FormEvent) {
        e.preventDefault()
        const euros = parseFloat(amount.replace(',', '.'))
        if (!label.trim() || isNaN(euros) || euros <= 0) return
        const cents = Math.round(euros * 100) * (type === 'expense' ? -1 : 1)
        await create.mutateAsync({
            label: label.trim(),
            amount_cents: cents,
            date,
            envelope_id: envelopeId || undefined,
        })
        onClose()
    }

    return (
        <Card className="mb-4">
            <CardContent className="p-4">
                <form onSubmit={handleSubmit} className="space-y-3">
                    <div className="flex gap-1 rounded-lg bg-muted p-1 w-fit">
                        {(['expense', 'income'] as const).map(t => (
                            <button
                                key={t}
                                type="button"
                                onClick={() => setType(t)}
                                className={cn(
                                    'rounded-md px-3 py-1 text-sm transition-colors',
                                    type === t
                                        ? 'bg-background text-foreground shadow-sm font-medium'
                                        : 'text-muted-foreground hover:text-foreground'
                                )}
                            >
                                {t === 'expense' ? 'Dépense' : 'Revenu'}
                            </button>
                        ))}
                    </div>
                    <div className="grid grid-cols-2 gap-3">
                        <div className="space-y-1.5">
                            <Label className="text-xs">Libellé</Label>
                            <Input
                                autoFocus
                                placeholder="Ex: Leclerc"
                                value={label}
                                onChange={e => setLabel(e.target.value)}
                            />
                        </div>
                        <div className="space-y-1.5">
                            <Label className="text-xs">Montant (€)</Label>
                            <Input
                                placeholder="42,50"
                                value={amount}
                                onChange={e => setAmount(e.target.value)}
                                inputMode="decimal"
                            />
                        </div>
                        <div className="space-y-1.5">
                            <Label className="text-xs">Date</Label>
                            <Input
                                type="date"
                                value={date}
                                onChange={e => setDate(e.target.value)}
                            />
                        </div>
                        <div className="space-y-1.5">
                            <Label className="text-xs">Enveloppe</Label>
                            <select
                                value={envelopeId}
                                onChange={e => setEnvelopeId(e.target.value)}
                                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring"
                            >
                                <option value="">Non catégorisée</option>
                                {envelopes.map(env => (
                                    <option key={env.id} value={env.id}>
                                        {env.icon} {env.name}
                                    </option>
                                ))}
                            </select>
                        </div>
                    </div>
                    <div className="flex gap-2 justify-end">
                        <Button type="button" variant="ghost" size="sm" onClick={onClose}>
                            Annuler
                        </Button>
                        <Button type="submit" size="sm" disabled={!label.trim() || !amount || create.isPending}>
                            Ajouter
                        </Button>
                    </div>
                </form>
            </CardContent>
        </Card>
    )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export function TransactionsPage() {
    const [search, setSearch] = useState('')
    const [adding, setAdding] = useState(false)
    const [sourceFilter, setSourceFilter] = useState<'all' | 'bank' | 'manual'>('all')

    const params: Record<string, string> = {}
    if (sourceFilter !== 'all') params.source = sourceFilter

    const { data, isLoading } = useTransactions(params)
    const { data: budgetData } = useCurrentBudget()

    const envelopes = budgetData?.envelopes ?? []

    // Client-side label search
    const transactions: Transaction[] = (data?.transactions ?? []).filter(t =>
        !search || t.clean_label.toLowerCase().includes(search.toLowerCase()) ||
        t.label.toLowerCase().includes(search.toLowerCase())
    )

    // Group by date
    const grouped = transactions.reduce<Record<string, Transaction[]>>((acc, tx) => {
        const d = tx.transaction_date
        if (!acc[d]) acc[d] = []
        acc[d].push(tx)
        return acc
    }, {})

    const sortedDates = Object.keys(grouped).sort((a, b) => b.localeCompare(a))

    // Totals for the visible set
    const totalExpenses = transactions.filter(t => t.amount_cents < 0).reduce((s, t) => s + t.amount_cents, 0)
    const totalIncome = transactions.filter(t => t.amount_cents > 0).reduce((s, t) => s + t.amount_cents, 0)

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Transactions"
                subtitle={`${data?.count ?? 0} transaction${(data?.count ?? 0) !== 1 ? 's' : ''}`}
                actions={
                    <Button size="sm" onClick={() => setAdding(true)}>
                        <Plus size={14} className="mr-1" /> Ajouter
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6 space-y-4 max-w-3xl">
                {/* Summary row */}
                <div className="grid grid-cols-2 gap-4">
                    <Card>
                        <CardContent className="p-4 flex items-center gap-3">
                            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-red-500/10">
                                <ArrowDownLeft size={16} className="text-red-500" />
                            </div>
                            <div>
                                <p className="text-xs text-muted-foreground">Dépenses</p>
                                <p className="text-lg font-semibold tabular-nums text-red-500">
                                    {fmt(Math.abs(totalExpenses))}
                                </p>
                            </div>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardContent className="p-4 flex items-center gap-3">
                            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-emerald-500/10">
                                <ArrowUpRight size={16} className="text-emerald-500" />
                            </div>
                            <div>
                                <p className="text-xs text-muted-foreground">Revenus</p>
                                <p className="text-lg font-semibold tabular-nums text-emerald-600 dark:text-emerald-400">
                                    {fmt(totalIncome)}
                                </p>
                            </div>
                        </CardContent>
                    </Card>
                </div>

                {/* Filters */}
                <div className="flex gap-2">
                    <div className="relative flex-1">
                        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            placeholder="Rechercher…"
                            value={search}
                            onChange={e => setSearch(e.target.value)}
                            className="pl-9"
                        />
                    </div>
                    <div className="flex gap-1 rounded-lg bg-muted p-1">
                        {(['all', 'bank', 'manual'] as const).map(s => (
                            <button
                                key={s}
                                onClick={() => setSourceFilter(s)}
                                className={cn(
                                    'rounded-md px-3 py-1 text-xs transition-colors',
                                    sourceFilter === s
                                        ? 'bg-background text-foreground shadow-sm font-medium'
                                        : 'text-muted-foreground hover:text-foreground'
                                )}
                            >
                                {s === 'all' ? 'Toutes' : s === 'bank' ? 'Banque' : 'Manuelles'}
                            </button>
                        ))}
                    </div>
                </div>

                {/* Add form */}
                {adding && (
                    <AddTransactionPanel
                        envelopes={envelopes}
                        onClose={() => setAdding(false)}
                    />
                )}

                {/* Transaction list grouped by date */}
                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}

                {!isLoading && transactions.length === 0 && (
                    <div className="flex flex-col items-center justify-center py-16 text-center">
                        <ArrowDownLeft size={40} className="mb-3 text-muted-foreground/30" />
                        <p className="text-sm text-muted-foreground">Aucune transaction trouvée</p>
                    </div>
                )}

                {sortedDates.map(date => (
                    <div key={date}>
                        <p className="mb-1.5 text-xs font-medium uppercase tracking-wider text-muted-foreground">
                            {fmtDate(date)}
                        </p>
                        <div className="space-y-1">
                            {grouped[date].map((tx: Transaction) => {
                                const isExpense = tx.amount_cents < 0
                                const env = envelopes.find(e => e.id === tx.envelope_id)
                                return (
                                    <div
                                        key={tx.id}
                                        className="flex items-center gap-3 rounded-lg border border-transparent px-3 py-2.5 hover:border-border hover:bg-muted/30 transition-all"
                                    >
                                        <div className={cn(
                                            'flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm',
                                            isExpense ? 'bg-red-500/10' : 'bg-emerald-500/10'
                                        )}>
                                            {env ? env.icon : isExpense ? '↓' : '↑'}
                                        </div>
                                        <div className="flex-1 min-w-0">
                                            <p className="truncate text-sm font-medium">
                                                {tx.clean_label || tx.label}
                                            </p>
                                            <div className="flex items-center gap-2 mt-0.5">
                                                {env && (
                                                    <span className="text-[10px] text-muted-foreground">{env.name}</span>
                                                )}
                                                {tx.source === 'bank' && (
                                                    <span className="text-[10px] text-muted-foreground/60">· banque</span>
                                                )}
                                            </div>
                                        </div>
                                        <span className={cn(
                                            'shrink-0 font-medium tabular-nums text-sm',
                                            isExpense ? 'text-red-500' : 'text-emerald-600 dark:text-emerald-400'
                                        )}>
                                            {isExpense ? '' : '+'}{fmt(tx.amount_cents)}
                                        </span>
                                    </div>
                                )
                            })}
                        </div>
                    </div>
                ))}
            </div>
        </div>
    )
}