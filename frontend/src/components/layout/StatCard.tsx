import { cn } from "@/lib/utils"
import { Card, CardContent } from "@/components/ui/card"

interface Props {
    label: string
    value: string | number
    sub?: string
    icon?: React.ReactNode
    trend?: "up" | "down" | "neutral"
    className?: string
}

export function StatCard({ label, value, sub, icon, className }: Props) {
    return (
        <Card className={cn("", className)}>
            <CardContent className="p-4">
                <div className="flex items-start justify-between">
                    <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        {label}
                    </p>
                    {icon && (
                        <span className="text-muted-foreground">{icon}</span>
                    )}
                </div>
                <p className="mt-2 text-2xl font-semibold tabular-nums text-foreground">
                    {value}
                </p>
                {sub && (
                    <p className="mt-0.5 text-xs text-muted-foreground">{sub}</p>
                )}
            </CardContent>
        </Card>
    )
}