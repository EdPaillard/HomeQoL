import { cn } from "@/lib/utils"
import { useNav, type NavSection } from "@/store/navigation"
import { useAlerts } from "@/hooks/useApi"
import {
    Home, CheckSquare, Target, Activity,
    Thermometer, Zap, Bell, Wallet,
    ArrowLeftRight, Building2, Film, Download,
    ChevronRight,
} from "lucide-react"

// ── Types ─────────────────────────────────────────────────────────────────────

interface NavItem {
    id: NavSection
    label: string
    icon: React.ReactNode
}

interface NavGroup {
    label: string
    items: NavItem[]
}

const NAV_GROUPS: NavGroup[] = [
    {
        label: "Aujourd'hui",
        items: [
            { id: "today", label: "Today", icon: <Home size={15} /> },
        ],
    },
    {
        label: "Planning",
        items: [
            { id: "planning.tasks", label: "Tâches", icon: <CheckSquare size={15} /> },
            { id: "planning.habits", label: "Habitudes", icon: <Activity size={15} /> },
            { id: "planning.goals", label: "Objectifs", icon: <Target size={15} /> },
        ],
    },
    {
        label: "Maison",
        items: [
            { id: "home.dashboard", label: "Vue d'ensemble", icon: <Thermometer size={15} /> },
            { id: "home.energy", label: "Énergie", icon: <Zap size={15} /> },
            { id: "home.alerts", label: "Alertes", icon: <Bell size={15} /> },
        ],
    },
    {
        label: "Budget",
        items: [
            { id: "budget.overview", label: "Enveloppes", icon: <Wallet size={15} /> },
            { id: "budget.transactions", label: "Transactions", icon: <ArrowLeftRight size={15} /> },
            { id: "budget.accounts", label: "Comptes", icon: <Building2 size={15} /> },
        ],
    },
    {
        label: "Médias",
        items: [
            { id: "media.library", label: "Bibliothèque", icon: <Film size={15} /> },
            { id: "media.downloads", label: "Téléchargements", icon: <Download size={15} /> },
        ],
    },
]

// ── Component ─────────────────────────────────────────────────────────────────

export function Sidebar() {
    const { active, setActive } = useNav()
    const { data: alertData } = useAlerts(true)
    const unreadAlerts = alertData?.alerts.length ?? 0

    return (
        <aside className="flex h-full w-56 flex-col border-r border-sidebar-border bg-sidebar">
            {/* Logo / brand */}
            {/* <div className="flex h-14 items-center gap-2.5 border-b border-sidebar-border px-4">
                <div className="flex h-7 w-7 items-center justify-center rounded-md bg-sidebar-primary text-sidebar-primary-foreground">
                    <Home size={14} />
                </div>
                <span className="text-sm font-semibold tracking-tight text-sidebar-foreground">
                    HomeQoL
                </span>
            </div> */}

            {/* Nav groups */}
            <nav className="flex-1 overflow-y-auto py-3">
                {NAV_GROUPS.map((group) => (
                    <div key={group.label} className="mb-4 px-3">
                        <p className="mb-1 px-2 text-[10px] font-medium uppercase tracking-widest text-muted-foreground">
                            {group.label}
                        </p>
                        {group.items.map((item) => {
                            const isActive = active === item.id
                            const badge =
                                item.id === "home.alerts" && unreadAlerts > 0
                                    ? unreadAlerts
                                    : null

                            return (
                                <button
                                    key={item.id}
                                    onClick={() => setActive(item.id)}
                                    className={cn(
                                        "flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-sm transition-colors",
                                        isActive
                                            ? "bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                                            : "text-sidebar-foreground hover:bg-sidebar-accent/50 hover:text-sidebar-accent-foreground"
                                    )}
                                >
                                    <span className="shrink-0 text-muted-foreground">{item.icon}</span>
                                    <span className="flex-1 text-left">{item.label}</span>
                                    {badge && (
                                        <span className="flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-medium text-white">
                                            {badge}
                                        </span>
                                    )}
                                    {isActive && (
                                        <ChevronRight size={12} className="shrink-0 text-muted-foreground" />
                                    )}
                                </button>
                            )
                        })}
                    </div>
                ))}
            </nav>

            {/* Footer */}
            <div className="border-t border-sidebar-border px-4 py-3">
                <p className="text-[11px] text-muted-foreground">
                    {new Date().toLocaleDateString("fr-FR", {
                        weekday: "long",
                        day: "numeric",
                        month: "long",
                    })}
                </p>
            </div>
        </aside>
    )
}