interface Props {
    title: string
    subtitle?: string
    actions?: React.ReactNode
}

export function PageHeader({ title, subtitle, actions }: Props) {
    return (
        <div className="flex items-start justify-between border-b border-border px-6 py-5">
            <div>
                <h1 className="text-xl font-semibold tracking-tight text-foreground">{title}</h1>
                {subtitle && (
                    <p className="mt-0.5 text-sm text-muted-foreground">{subtitle}</p>
                )}
            </div>
            {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
    )
}