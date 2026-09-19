import { Thermometer, RefreshCw } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/layout/PageHeader'
import { useHomeDashboard } from '@/hooks/useApi'
import { useNav } from '@/store/navigation'
import { cn } from '@/lib/utils'
import type { TemperatureReading, ShutterReading } from '@/types'

function tempColor(c: number) {
    if (c < 16) return 'text-blue-500'
    if (c < 20) return 'text-cyan-500'
    if (c < 24) return 'text-emerald-500'
    if (c < 27) return 'text-yellow-500'
    return 'text-red-500'
}

export function HomeDashboardPage() {
    const { data, isLoading, refetch, isFetching } = useHomeDashboard()
    const { setActive } = useNav()
    const temps = data?.latest_temperatures ?? []
    const shutters = data?.latest_shutters ?? []
    const interior = temps.filter(t => t.source === 'interior')
    const exterior = temps.find(t => t.source === 'exterior')

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Maison"
                subtitle={data ? `Mis à jour à ${new Date(data.generated_at).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}` : 'Chargement…'}
                actions={
                    <div className="flex gap-2">
                        <Button variant="outline" size="sm" onClick={() => setActive('home.energy')}>Énergie →</Button>
                        <Button variant="outline" size="icon" className="h-8 w-8" onClick={() => refetch()} disabled={isFetching}>
                            <RefreshCw size={14} className={cn(isFetching && 'animate-spin')} />
                        </Button>
                    </div>
                }
            />
            <div className="flex-1 overflow-y-auto p-6 space-y-6 max-w-5xl">
                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}
                {data && (
                    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                        <Card><CardContent className="p-4"><p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Pièces</p><p className="mt-2 text-2xl font-semibold">{data.rooms.length}</p></CardContent></Card>
                        <Card><CardContent className="p-4"><p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Extérieur</p><p className={cn('mt-2 text-2xl font-semibold tabular-nums', exterior ? tempColor(exterior.celsius) : '')}>{exterior ? `${exterior.celsius.toFixed(1)}°C` : '—'}</p></CardContent></Card>
                        <Card><CardContent className="p-4"><p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Luminosité</p><p className="mt-2 text-2xl font-semibold tabular-nums">{data.latest_sunlight ? `${Math.round(data.latest_sunlight.lux)} lux` : '—'}</p></CardContent></Card>
                        <Card><CardContent className="p-4"><p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">Conso. hier</p><p className="mt-2 text-2xl font-semibold tabular-nums">{data.today_kwh != null ? `${data.today_kwh.toFixed(1)} kWh` : '—'}</p></CardContent></Card>
                    </div>
                )}
                {interior.length > 0 && (
                    <div>
                        <h2 className="mb-3 text-sm font-medium uppercase tracking-wider text-muted-foreground">Températures</h2>
                        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
                            {interior.map((t: TemperatureReading) => (
                                <Card key={t.id}>
                                    <CardContent className="p-4">
                                        <div className="flex items-start justify-between">
                                            <p className="text-xs text-muted-foreground truncate max-w-[80%]">{t.room_name}</p>
                                            <Thermometer size={14} className={cn('shrink-0', tempColor(t.celsius))} />
                                        </div>
                                        <p className={cn('mt-2 text-3xl font-semibold tabular-nums', tempColor(t.celsius))}>{t.celsius.toFixed(1)}°</p>
                                        <p className="mt-1 text-[10px] text-muted-foreground">{new Date(t.recorded_at).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}</p>
                                    </CardContent>
                                </Card>
                            ))}
                        </div>
                    </div>
                )}
                {shutters.length > 0 && (
                    <div>
                        <h2 className="mb-3 text-sm font-medium uppercase tracking-wider text-muted-foreground">Volets</h2>
                        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
                            {shutters.map((s: ShutterReading) => (
                                <Card key={s.id}>
                                    <CardContent className="p-4">
                                        <p className="text-xs text-muted-foreground truncate">{s.room_name}</p>
                                        <div className="mt-3 h-16 w-full rounded border border-border bg-muted overflow-hidden relative">
                                            <div className="absolute top-0 left-0 right-0 bg-foreground/20 transition-all" style={{ height: `${100 - s.position}%` }} />
                                        </div>
                                        <p className="mt-1.5 text-center text-xs font-medium">{s.position === 0 ? 'Fermé' : s.position === 100 ? 'Ouvert' : `${s.position}%`}</p>
                                    </CardContent>
                                </Card>
                            ))}
                        </div>
                    </div>
                )}
                {!isLoading && !data && (
                    <div className="flex flex-col items-center justify-center py-24 text-center">
                        <Thermometer size={48} className="mb-4 text-muted-foreground/30" />
                        <p className="text-sm font-medium text-muted-foreground">Aucune donnée</p>
                        <p className="mt-1 text-xs text-muted-foreground">Vérifiez que tydom2mqtt est en cours d'exécution.</p>
                    </div>
                )}
            </div>
        </div>
    )
}