import { Building2, RefreshCw, CreditCard, PiggyBank, ArrowRightLeft } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PageHeader } from '@/components/layout/PageHeader'
import { useBankAccounts } from '@/hooks/useApi'
import { cn } from '@/lib/utils'
import type { BankAccount } from '@/types'

function fmt(cents: number) {
    return (cents / 100).toLocaleString('fr-FR', { style: 'currency', currency: 'EUR' })
}

function AccountIcon({ type }: { type: string }) {
    switch (type) {
        case 'savings': return <PiggyBank size={18} />
        case 'credit': return <CreditCard size={18} />
        default: return <ArrowRightLeft size={18} />
    }
}

function AccountTypeLabel(type: string) {
    const labels: Record<string, string> = {
        checking: 'Compte courant',
        savings: 'Livret',
        credit: 'Carte de crédit',
        other: 'Autre',
    }
    return labels[type] ?? type
}

export function AccountsPage() {
    const { data, isLoading, refetch, isFetching } = useBankAccounts()

    const accounts: BankAccount[] = data?.accounts ?? []

    const totalBalance = accounts
        .filter(a => a.type !== 'credit')
        .reduce((s, a) => s + a.balance_cents, 0)

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Comptes bancaires"
                subtitle="Synchronisé via Powens · Caisse d'Épargne"
                actions={
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={() => refetch()}
                        disabled={isFetching}
                    >
                        <RefreshCw size={13} className={cn('mr-1.5', isFetching && 'animate-spin')} />
                        Actualiser
                    </Button>
                }
            />

            <div className="flex-1 overflow-y-auto p-6 space-y-6 max-w-3xl">
                {isLoading && <p className="text-sm text-muted-foreground">Chargement…</p>}

                {/* Net worth card */}
                {accounts.length > 0 && (
                    <Card className="border-primary/20 bg-primary/5">
                        <CardContent className="p-5">
                            <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                                Patrimoine total (hors crédit)
                            </p>
                            <p className={cn(
                                'mt-2 text-4xl font-bold tabular-nums',
                                totalBalance >= 0 ? 'text-foreground' : 'text-destructive'
                            )}>
                                {fmt(totalBalance)}
                            </p>
                            {data?.accounts?.[0]?.last_synced_at && (
                                <p className="mt-1.5 text-xs text-muted-foreground">
                                    Dernière synchro : {new Date(data.accounts[0].last_synced_at).toLocaleString('fr-FR', {
                                        day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit'
                                    })}
                                </p>
                            )}
                        </CardContent>
                    </Card>
                )}

                {/* Account list */}
                {accounts.length === 0 && !isLoading && (
                    <div className="flex flex-col items-center justify-center py-24 text-center">
                        <Building2 size={48} className="mb-4 text-muted-foreground/30" />
                        <p className="text-sm font-medium text-muted-foreground">Aucun compte synchronisé</p>
                        <p className="mt-1 text-xs text-muted-foreground max-w-xs">
                            Configurez Powens avec votre token dans le .env pour importer vos comptes Caisse d'Épargne.
                        </p>
                    </div>
                )}

                <div className="space-y-3">
                    {accounts.map((account: BankAccount) => {
                        const isNegative = account.balance_cents < 0
                        return (
                            <Card key={account.id}>
                                <CardContent className="p-4">
                                    <div className="flex items-center gap-4">
                                        {/* Icon */}
                                        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
                                            <AccountIcon type={account.type} />
                                        </div>

                                        {/* Info */}
                                        <div className="flex-1 min-w-0">
                                            <p className="font-medium text-sm truncate">{account.label}</p>
                                            <div className="flex items-center gap-2 mt-0.5">
                                                <span className="text-xs text-muted-foreground">
                                                    {AccountTypeLabel(account.type)}
                                                </span>
                                                {account.iban && (
                                                    <span className="text-[10px] text-muted-foreground font-mono">
                                                        ···{account.iban.slice(-4)}
                                                    </span>
                                                )}
                                            </div>
                                        </div>

                                        {/* Balance */}
                                        <div className="text-right shrink-0">
                                            <p className={cn(
                                                'text-lg font-semibold tabular-nums',
                                                isNegative ? 'text-destructive' : 'text-foreground'
                                            )}>
                                                {fmt(account.balance_cents)}
                                            </p>
                                            <p className="text-[10px] text-muted-foreground uppercase tracking-wide">
                                                {account.currency}
                                            </p>
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>
                        )
                    })}
                </div>
            </div>
        </div>
    )
}