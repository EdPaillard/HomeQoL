import { Wallet, TrendingUp } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/layout/PageHeader'
import { useCurrentBudget } from '@/hooks/useApi'
import { useNav } from '@/store/navigation'
import { cn } from '@/lib/utils'
import type { Envelope } from '@/types'

function fmt(cents: number) {
    return (cents / 100).toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}

export function BudgetPage() {
    const { data, isLoading } = useCurrentBudget()
    const { setActive } = useNav()
    const now = new Date()
    const monthLabel = now.toLocaleDateString('fr-FR', { month: 'long', year: 'numeric' })

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Budget"
                subtitle={monthLabel.charAt(0).toUpperCase() + monthLabel.slice(1)}
                actions={
                    <Button variant="outline" size="sm" onClick={() => setActive('budget.transactions')}>
                        Transactions →
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6 space-y-6 max-w-4xl">
                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}

                {data && (
                    <>
                        {/* Summary */}
                        <div className="grid grid-cols-3 gap-4">
                            <Card>
                                <CardContent className="p-4">
                                    <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Dépensé</p>
                                    <p className="mt-2 text-2xl font-semibold tabular-nums">{fmt(data.summary.total_spent_cents)}</p>
                                    <p className="text-xs text-muted-foreground mt-0.5">sur {fmt(data.summary.total_budgeted_cents)}</p>
                                </CardContent>
                            </Card>
                            <Card className={cn(data.summary.remaining_cents < 0 ? 'border-destructive/50' : '')}>
                                <CardContent className="p-4">
                                    <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Restant</p>
                                    <p className={cn('mt-2 text-2xl font-semibold tabular-nums', data.summary.remaining_cents < 0 ? 'text-destructive' : 'text-emerald-600 dark:text-emerald-400')}>
                                        {fmt(data.summary.remaining_cents)}
                                    </p>
                                    <p className="text-xs text-muted-foreground mt-0.5">
                                        {Math.round((data.summary.total_spent_euros / data.summary.total_budgeted_euros) * 100)}% du budget utilisé
                                    </p>
                                </CardContent>
                            </Card>
                            <Card>
                                <CardContent className="p-4">
                                    <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Enveloppes</p>
                                    <p className="mt-2 text-2xl font-semibold">{data.envelopes.length}</p>
                                    <p className="text-xs text-muted-foreground mt-0.5">
                                        {data.envelopes.filter(e => e.percent_used > 100).length} dépassée(s)
                                    </p>
                                </CardContent>
                            </Card>
                        </div>

                        {/* Global progress bar */}
                        <div className="space-y-1.5">
                            <div className="flex justify-between text-xs text-muted-foreground">
                                <span>Progression mensuelle</span>
                                <span>{Math.round((data.summary.total_spent_euros / data.summary.total_budgeted_euros) * 100)}%</span>
                            </div>
                            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                                <div
                                    className={cn('h-full rounded-full transition-all', data.summary.remaining_cents < 0 ? 'bg-destructive' : 'bg-primary')}
                                    style={{ width: `${Math.min((data.summary.total_spent_euros / data.summary.total_budgeted_euros) * 100, 100)}%` }}
                                />
                            </div>
                        </div>

                        {/* Envelopes grid */}
                        <div>
                            <h2 className="mb-3 text-sm font-medium uppercase tracking-wider text-muted-foreground">Enveloppes</h2>
                            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                                {data.envelopes.map((env: Envelope) => {
                                    const over = env.percent_used > 100
                                    const warn = env.percent_used > 80 && !over
                                    return (
                                        <Card key={env.id} className={cn(over && 'border-destructive/40')}>
                                            <CardContent className="p-4">
                                                <div className="flex items-start justify-between gap-2">
                                                    <div className="flex items-center gap-2 min-w-0">
                                                        <span className="text-lg shrink-0">{env.icon}</span>
                                                        <p className="text-sm font-medium truncate">{env.name}</p>
                                                    </div>
                                                    {over && <span className="text-[10px] font-medium text-destructive shrink-0">Dépassé</span>}
                                                </div>

                                                <div className="mt-3">
                                                    <div className="flex justify-between text-xs text-muted-foreground mb-1.5">
                                                        <span className="tabular-nums">{fmt(env.spent_cents)}</span>
                                                        <span className="tabular-nums">{fmt(env.budgeted_cents)}</span>
                                                    </div>
                                                    <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                                                        <div
                                                            className={cn(
                                                                'h-full rounded-full transition-all',
                                                                over ? 'bg-destructive' : warn ? 'bg-yellow-500' : 'bg-primary'
                                                            )}
                                                            style={{ background: !over && !warn ? env.color : undefined, width: `${Math.min(env.percent_used, 100)}%` }}
                                                        />
                                                    </div>
                                                    <p className="mt-1 text-right text-[10px] text-muted-foreground tabular-nums">
                                                        {Math.round(env.percent_used)}% · reste {fmt(env.remaining_cents)}
                                                    </p>
                                                </div>
                                            </CardContent>
                                        </Card>
                                    )
                                })}
                            </div>
                        </div>
                    </>
                )}
            </div>
        </div>
    )
}