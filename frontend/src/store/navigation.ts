import { create } from "zustand"

export type NavSection =
    | "today"
    | "planning.tasks"
    | "planning.habits"
    | "planning.goals"
    | "home.dashboard"
    | "home.energy"
    | "home.alerts"
    | "budget.overview"
    | "budget.transactions"
    | "budget.accounts"
    | "media.library"
    | "media.downloads"

interface NavStore {
    active: NavSection
    setActive: (section: NavSection) => void
}

export const useNav = create<NavStore>((set) => ({
    active: "today",
    setActive: (section) => set({ active: section }),
}))