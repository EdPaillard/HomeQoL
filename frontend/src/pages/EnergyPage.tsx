import { useState } from 'react'
import { Zap, TrendingDown, TrendingUp, ExternalLink } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/layout/PageHeader'
import { useEnergyHistory, useHomeDashboard } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { EnergyReading } from '@/types'

const RANGE_TABS = [
    { id: '7', label: '7 jours' },
    { id: '30', label: '30 jours' },
    { id: '90', label: '3 mois' },
]

function dateRange(days: number) {
    const to = new Date()
    const from = new Date()
    from.setDate(from.getDate() - days)
    return {
        from: from.toISOString().split('T')[0],
        to: to.toISOString().split('T')[0],
    }
}

export function EnergyPage() {
    const [range, setRange] = useState('30')
    const { from, to } = dateRange(parseInt(range))
    const { data, isLoading } = useEnergyHistory(from, to, 'daily')
    const { data: homeData } = useHomeDashboard()

    const readings: EnergyReading[] = data?.readings ?? []
    const total = readings.reduce((s, r) => s + r.kwh, 0)
    const avg = readings.length > 0 ? total / readings.length : 0
    const max = readings.length > 0 ? Math.max(...readings.map(r => r.kwh)) : 0
    const cost = total * 0.2516 // adjust to your EDF rate

    // Simple bar chart — max value for scaling
    const chartMax = max > 0 ? max : 1

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Énergie"
                subtitle="Consommation Linky · données J-1"
                actions={
                    <Button variant="outline" size="sm" asChild>
                        <a href="/grafana" target="_blank" rel="noopener noreferrer">
                            <ExternalLink size={13} className="mr-1.5" /> Grafana
                        </a>
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6 space-y-6 max-w-4xl">
                {/* Range selector */}
                <div className="flex gap-1 rounded-lg bg-muted p-1 w-fit">
                    {RANGE_TABS.map(tab => (
                        <button
                            key={tab.id}
                            onClick={() => setRange(tab.id)}
                            className={cn(
                                'rounded-md px-3 py-1.5 text-sm transition-colors',
                                range === tab.id
                                    ? 'bg-background text-foreground shadow-sm font-medium'
                                    : 'text-muted-foreground hover:text-foreground'
                            )}
                        >
                            {tab.label}
                        </button>
                    ))}
                </div>

                {/* KPI row */}
                <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                    <Card>
                        <CardContent className="p-4">
                            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Total</p>
                            <p className="mt-2 text-2xl font-semibold tabular-nums">{total.toFixed(1)} <span className="text-sm font-normal text-muted-foreground">kWh</span></p>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardContent className="p-4">
                            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Moyenne / jour</p>
                            <p className="mt-2 text-2xl font-semibold tabular-nums">{avg.toFixed(1)} <span className="text-sm font-normal text-muted-foreground">kWh</span></p>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardContent className="p-4">
                            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Pic journalier</p>
                            <p className="mt-2 text-2xl font-semibold tabular-nums">{max.toFixed(1)} <span className="text-sm font-normal text-muted-foreground">kWh</span></p>
                        </CardContent>
                    </Card>
                    <Card>
                        <CardContent className="p-4">
                            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Coût estimé</p>
                            <p className="mt-2 text-2xl font-semibold tabular-nums">{cost.toFixed(2)} <span className="text-sm font-normal text-muted-foreground">€</span></p>
                            <p className="text-[10px] text-muted-foreground">@ 0,2516 €/kWh</p>
                        </CardContent>
                    </Card>
                </div>

                {/* Bar chart */}
                <Card>
                    <CardHeader className="pb-2">
                        <CardTitle className="text-sm font-medium">Consommation journalière</CardTitle>
                    </CardHeader>
                    <CardContent>
                        {isLoading ? (
                            <p className="text-sm text-muted-foreground py-8 text-center">Chargement…</p>
                        ) : readings.length === 0 ? (
                            <div className="flex flex-col items-center justify-center py-12 text-center">
                                <Zap size={36} className="mb-3 text-muted-foreground/30" />
                                <p className="text-sm text-muted-foreground">Aucune donnée sur cette période</p>
                                <p className="text-xs text-muted-foreground mt-1">Vérifiez votre token Conso API dans le .env</p>
                            </div>
                        ) : (
                            <div className="mt-2">
                                {/* Bars */}
                                <div className="flex items-end gap-0.5 h-40">
                                    {readings.map((r: EnergyReading, i) => {
                                        const heightPct = (r.kwh / chartMax) * 100
                                        const isHigh = r.kwh > avg * 1.3
                                        return (
                                            <div
                                                key={r.id ?? i}
                                                className="group flex-1 flex flex-col items-center justify-end"
                                                title={`${r.reading_date} : ${r.kwh.toFixed(2)} kWh`}
                                            >
                                                <div
                                                    className={cn(
                                                        'w-full rounded-t-sm transition-all',
                                                        isHigh ? 'bg-yellow-500/70' : 'bg-primary/60',
                                                        'group-hover:opacity-100 opacity-80'
                                                    )}
                                                    style={{ height: `${heightPct}%`, minHeight: '2px' }}
                                                />
                                            </div>
                                        )
                                    })}
                                </div>
                                {/* X-axis labels — show first, middle, last */}
                                <div className="flex justify-between mt-1 text-[10px] text-muted-foreground">
                                    <span>{readings[0]?.reading_date ? new Date(readings[0].reading_date).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' }) : ''}</span>
                                    <span>{avg.toFixed(1)} kWh moy.</span>
                                    <span>{readings[readings.length - 1]?.reading_date ? new Date(readings[readings.length - 1].reading_date).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' }) : ''}</span>
                                </div>
                            </div>
                        )}
                    </CardContent>
                </Card>

                {/* Temperature correlation note */}
                {homeData?.latest_temperatures && homeData.latest_temperatures.length > 0 && (
                    <Card className="border-dashed">
                        <CardContent className="p-4">
                            <p className="text-xs text-muted-foreground">
                                💡 Corrélation : pour des analyses avancées (température vs consommation), ouvrez{' '}
                                <a href="/grafana" target="_blank" rel="noopener noreferrer" className="text-primary underline">Grafana</a>.
                            </p>
                        </CardContent>
                    </Card>
                )}
            </div>
        </div>
    )
}