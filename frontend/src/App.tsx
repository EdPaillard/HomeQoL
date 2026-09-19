import React, { useState } from 'react'
import { Lock, Sparkles, Bell, LogOut } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Sidebar } from '@/components/layout/Sidebar'
import { useNav } from '@/store/navigation'

// Pages
import { TodayPage } from '@/pages/TodayPage'
import { TasksPage } from '@/pages/TasksPage'
import { HabitsPage } from '@/pages/HabitsPage'
import { GoalsPage } from '@/pages/GoalsPage'
import { HomeDashboardPage } from '@/pages/HomeDashboardPage'
import { EnergyPage } from '@/pages/EnergyPage'
import { AlertsPage } from '@/pages/AlertsPage'
import { BudgetPage } from '@/pages/BudgetPage'
import { TransactionsPage } from '@/pages/TransactionsPage'
import { AccountsPage } from '@/pages/AccountsPage'
import { MediaPage } from '@/pages/MediaPage'

// ── Page router ───────────────────────────────────────────────────────────────

function PageRouter() {
  const { active } = useNav()

  switch (active) {
    case 'today': return <TodayPage />
    case 'planning.tasks': return <TasksPage />
    case 'planning.habits': return <HabitsPage />
    case 'planning.goals': return <GoalsPage />
    case 'home.dashboard': return <HomeDashboardPage />
    case 'home.energy': return <EnergyPage />
    case 'home.alerts': return <AlertsPage />
    case 'budget.overview': return <BudgetPage />
    case 'budget.transactions': return <TransactionsPage />
    case 'budget.accounts': return <AccountsPage />
    case 'media.library':
    case 'media.downloads': return <MediaPage />
    default: return <TodayPage />
  }
}

// ── Login screen ──────────────────────────────────────────────────────────────

function LoginScreen({ onLogin }: { onLogin: () => void }) {
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onLogin()
  }

  return (
    <div className="min-h-screen w-full flex items-center justify-center bg-background text-foreground p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="space-y-1">
          <div className="flex items-center gap-2 mb-2">
            <Sparkles className="h-5 w-5 text-primary" />
            <CardTitle className="text-xl font-bold">HomeQoL</CardTitle>
          </div>
          <CardDescription>Connectez-vous pour accéder à votre tableau de bord.</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="email">Email</Label>
              <Input id="email" type="email" placeholder="alex@home.local" required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="password">Mot de passe</Label>
              <Input id="password" type="password" required />
            </div>
            <Button type="submit" className="w-full">
              <Lock className="mr-2 h-4 w-4" /> Se connecter
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

// ── App shell ─────────────────────────────────────────────────────────────────

export default function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false)

  if (!isAuthenticated) {
    return <LoginScreen onLogin={() => setIsAuthenticated(true)} />
  }

  return (
    <div className="flex h-screen flex-col bg-background text-foreground antialiased">
      {/* Top bar */}
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-border bg-card/50 px-4 backdrop-blur">
        <div className="flex items-center gap-2 text-lg font-semibold tracking-tight">
          <Sparkles className="h-5 w-5 text-primary" />
          <span>HomeQoL</span>
        </div>
        <div className="flex items-center gap-3">
          <Button variant="outline" size="icon" className="relative h-9 w-9">
            <Bell className="h-4 w-4" />
          </Button>
          <div className="flex items-center gap-2 border-l border-border pl-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/20 text-xs font-bold text-primary">
              ME
            </div>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground"
              onClick={() => setIsAuthenticated(false)}
            >
              <LogOut className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </header>

      {/* Body: sidebar + content */}
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />
        <main className="flex flex-1 flex-col overflow-hidden">
          <PageRouter />
        </main>
      </div>
    </div>
  )
}