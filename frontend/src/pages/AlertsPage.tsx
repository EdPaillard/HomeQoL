import { AlertTriangle, Bell, BellOff, CheckCheck } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/layout/PageHeader'
import { useAlerts, useAcknowledgeAlert } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import { useState } from 'react'
import type { Alert } from '@/types'

const SEVERITY_STYLES: Record<string, string> = {
    info: 'border-l-blue-400 bg-blue-50/50 dark:bg-blue-950/20',
    warning: 'border-l-yellow-400 bg-yellow-50/50 dark:bg-yellow-950/20',
    critical: 'border-l-red-500 bg-red-50/50 dark:bg-red-950/20',
}

const SEVERITY_ICON: Record<string, string> = {
    info: '💡', warning: '⚠️', critical: '🚨',
}

export function AlertsPage() {
    const [showAll, setShowAll] = useState(false)
    const { data, isLoading } = useAlerts(!showAll)
    const acknowledge = useAcknowledgeAlert()

    const alerts: Alert[] = data?.alerts ?? []
    const unread = alerts.filter(a => !a.acknowledged_at).length

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Alertes"
                subtitle={`${unread} non lue${unread !== 1 ? 's' : ''}`}
                actions={
                    <div className="flex items-center gap-2">
                        <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setShowAll(!showAll)}
                        >
                            {showAll ? <Bell size={13} className="mr-1.5" /> : <BellOff size={13} className="mr-1.5" />}
                            {showAll ? 'Non lues seulement' : 'Toutes'}
                        </Button>
                        {unread > 0 && (
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => alerts.filter(a => !a.acknowledged_at).forEach(a => acknowledge.mutate(a.id))}
                                disabled={acknowledge.isPending}
                            >
                                <CheckCheck size={13} className="mr-1.5" /> Tout lire
                            </Button>
                        )}
                    </div>
                }
            />

            <div className="flex-1 overflow-y-auto p-6 space-y-3 max-w-3xl">
                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}

                {!isLoading && alerts.length === 0 && (
                    <div className="flex flex-col items-center justify-center py-24 text-center">
                        <Bell size={40} className="mb-4 text-muted-foreground/30" />
                        <p className="text-sm font-medium text-muted-foreground">Aucune alerte{!showAll ? ' non lue' : ''}</p>
                    </div>
                )}

                {alerts.map((alert: Alert) => (
                    <div
                        key={alert.id}
                        className={cn(
                            'border-l-4 rounded-lg border border-border p-4 transition-opacity',
                            SEVERITY_STYLES[alert.severity] ?? SEVERITY_STYLES.info,
                            alert.acknowledged_at && 'opacity-50'
                        )}
                    >
                        <div className="flex items-start gap-3">
                            <span className="text-lg shrink-0">{SEVERITY_ICON[alert.severity] ?? '💡'}</span>
                            <div className="flex-1 min-w-0">
                                <div className="flex items-start justify-between gap-2">
                                    <p className="text-sm font-medium">{alert.title}</p>
                                    <time className="text-[10px] text-muted-foreground shrink-0">
                                        {new Date(alert.created_at).toLocaleString('fr-FR', {
                                            day: 'numeric', month: 'short',
                                            hour: '2-digit', minute: '2-digit',
                                        })}
                                    </time>
                                </div>
                                {alert.body && <p className="mt-0.5 text-xs text-muted-foreground">{alert.body}</p>}
                            </div>
                            {!alert.acknowledged_at && (
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="shrink-0 h-7 text-xs"
                                    onClick={() => acknowledge.mutate(alert.id)}
                                    disabled={acknowledge.isPending}
                                >
                                    Lu
                                </Button>
                            )}
                        </div>
                    </div>
                ))}
            </div>
        </div>
    )
}