import { useState } from 'react'
import { Film, Tv, Download, ExternalLink, Search, Star, Clock, Loader2, Trash2, AlertCircle } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { PageHeader } from '@/components/layout/PageHeader'
import { useJellyfinUser, useMovies, useSeries, useTorrents, useRemoveTorrent } from '@/hooks/useApi'
import { mediaApi } from '@/api/client'
import { cn } from '@/lib/utils'
import type { JellyfinItem, Torrent } from '@/types'
import { useNav } from '@/store/navigation'

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtBytes(bytes: number) {
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
    if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
    return `${(bytes / 1024 ** 3).toFixed(2)} GB`
}

function fmtSpeed(bps: number) {
    if (bps < 1024) return `${bps} B/s`
    if (bps < 1024 * 1024) return `${(bps / 1024).toFixed(0)} KB/s`
    return `${(bps / 1024 ** 2).toFixed(1)} MB/s`
}

function fmtEta(secs: number) {
    if (secs < 0) return '—'
    if (secs < 60) return `${secs}s`
    if (secs < 3600) return `${Math.floor(secs / 60)}min`
    return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}min`
}

function fmtRuntime(ticks: number) {
    const mins = Math.floor(ticks / 600_000_000)
    return `${Math.floor(mins / 60)}h${mins % 60 > 0 ? ` ${mins % 60}min` : ''}`
}

const TORRENT_STATUS: Record<number, { label: string; color: string }> = {
    0: { label: 'Arrêté', color: 'text-muted-foreground' },
    1: { label: 'File d\'attente', color: 'text-yellow-500' },
    2: { label: 'Vérification', color: 'text-blue-500' },
    3: { label: 'File d\'attente', color: 'text-yellow-500' },
    4: { label: 'Téléchargement', color: 'text-emerald-500' },
    5: { label: 'File seeding', color: 'text-yellow-500' },
    6: { label: 'Seeding', color: 'text-blue-400' },
}

// ── Tabs ──────────────────────────────────────────────────────────────────────

type MediaTab = 'movies' | 'series' | 'downloads'

const TABS: { id: MediaTab; label: string; icon: React.ReactNode }[] = [
    { id: 'movies', label: 'Films', icon: <Film size={14} /> },
    { id: 'series', label: 'Séries', icon: <Tv size={14} /> },
    { id: 'downloads', label: 'Téléchargements', icon: <Download size={14} /> },
]

// ── Media card (movie or series) ──────────────────────────────────────────────

function MediaCard({ item }: { item: JellyfinItem }) {
    const poster = mediaApi.poster(item, 400)

    return (
        <div className="group relative overflow-hidden rounded-lg border border-border bg-card transition-all hover:border-border/80 hover:shadow-lg">
            {/* Poster */}
            <div className="aspect-[2/3] overflow-hidden bg-muted">
                {poster ? (
                    <img
                        src={poster}
                        alt={item.Name}
                        className="h-full w-full object-cover transition-transform group-hover:scale-105"
                        loading="lazy"
                    />
                ) : (
                    <div className="flex h-full items-center justify-center text-muted-foreground">
                        {item.Type === 'Movie' ? <Film size={32} /> : <Tv size={32} />}
                    </div>
                )}
            </div>

            {/* Overlay on hover */}
            <div className="absolute inset-0 flex flex-col justify-end bg-gradient-to-t from-black/80 via-black/20 to-transparent opacity-0 transition-opacity group-hover:opacity-100 p-3">
                {item.Overview && (
                    <p className="text-[11px] text-white/80 line-clamp-3 mb-1">{item.Overview}</p>
                )}
                <div className="flex items-center gap-2">
                    {item.CommunityRating && (
                        <span className="flex items-center gap-0.5 text-[11px] text-yellow-400">
                            <Star size={10} fill="currentColor" />
                            {item.CommunityRating.toFixed(1)}
                        </span>
                    )}
                    {item.RunTimeTicks && (
                        <span className="flex items-center gap-0.5 text-[11px] text-white/60">
                            <Clock size={10} />
                            {fmtRuntime(item.RunTimeTicks)}
                        </span>
                    )}
                </div>
            </div>

            {/* Info below */}
            <div className="p-2">
                <p className="truncate text-xs font-medium">{item.Name}</p>
                <p className="text-[10px] text-muted-foreground">
                    {item.ProductionYear ?? ''}
                    {item.OfficialRating ? ` · ${item.OfficialRating}` : ''}
                </p>
            </div>
        </div>
    )
}

// ── Library tab ───────────────────────────────────────────────────────────────

function LibraryTab({ type }: { type: 'movies' | 'series' }) {
    const [search, setSearch] = useState('')
    const { data: userdata, isLoading: userLoading, isError: userError } = useJellyfinUser()
    const userId = userdata?.Id ?? ''

    const { data: movies, isLoading: moviesLoading } = useMovies(userId)
    const { data: series, isLoading: seriesLoading } = useSeries(userId)

    const data = type === 'movies' ? movies : series
    const loading = userLoading || (type === 'movies' ? moviesLoading : seriesLoading)

    const items: JellyfinItem[] = (data?.Items ?? []).filter(item =>
        !search || item.Name.toLowerCase().includes(search.toLowerCase())
    )

    // Jellyfin not configured
    if (userError) {
        return (
            <div className="flex flex-col items-center justify-center py-24 text-center gap-3">
                <AlertCircle size={40} className="text-muted-foreground/40" />
                <div>
                    <p className="text-sm font-medium text-muted-foreground">Jellyfin non configuré</p>
                    <p className="mt-1 text-xs text-muted-foreground max-w-xs">
                        Ajoutez <code className="bg-muted px-1 rounded">VITE_JELLYFIN_API_KEY</code> dans votre <code className="bg-muted px-1 rounded">.env</code> et assurez-vous que Jellyfin est accessible via le proxy.
                    </p>
                </div>
                <Button variant="outline" size="sm" asChild>
                    <a href="/jellyfin" target="_blank" rel="noopener noreferrer">
                        <ExternalLink size={13} className="mr-1.5" /> Ouvrir Jellyfin
                    </a>
                </Button>
            </div>
        )
    }

    return (
        <div className="flex flex-col gap-4">
            {/* Search + stats */}
            <div className="flex items-center gap-3">
                <div className="relative flex-1 max-w-xs">
                    <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder="Rechercher…"
                        value={search}
                        onChange={e => setSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <span className="text-xs text-muted-foreground">
                    {data?.TotalRecordCount ?? 0} {type === 'movies' ? 'film(s)' : 'série(s)'}
                </span>
                <Button variant="outline" size="sm" asChild>
                    <a href="/jellyfin" target="_blank" rel="noopener noreferrer">
                        <ExternalLink size={13} className="mr-1.5" /> Jellyfin
                    </a>
                </Button>
            </div>

            {/* Grid */}
            {loading && (
                <div className="flex items-center justify-center py-16 gap-2 text-muted-foreground">
                    <Loader2 size={20} className="animate-spin" />
                    <span className="text-sm">Chargement de la bibliothèque…</span>
                </div>
            )}

            {!loading && items.length === 0 && (
                <div className="flex flex-col items-center justify-center py-16 text-center">
                    {type === 'movies' ? <Film size={40} className="mb-3 text-muted-foreground/30" /> : <Tv size={40} className="mb-3 text-muted-foreground/30" />}
                    <p className="text-sm text-muted-foreground">
                        {search ? 'Aucun résultat' : `Aucun ${type === 'movies' ? 'film' : 'série'} dans la bibliothèque`}
                    </p>
                    {!search && (
                        <p className="mt-1 text-xs text-muted-foreground">
                            Ajoutez une bibliothèque dans Jellyfin pointant vers /media
                        </p>
                    )}
                </div>
            )}

            <div className="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-7">
                {items.map(item => (
                    <MediaCard key={item.Id} item={item} />
                ))}
            </div>
        </div>
    )
}

// ── Downloads tab ─────────────────────────────────────────────────────────────

function DownloadsTab() {
    const { data, isLoading, isError } = useTorrents()
    const removeTorrent = useRemoveTorrent()
    const [confirmDelete, setConfirmDelete] = useState<number | null>(null)

    const torrents: Torrent[] = data?.torrents ?? []
    const active = torrents.filter(t => t.status === 4)
    const seeding = torrents.filter(t => t.status === 6)
    const stopped = torrents.filter(t => ![4, 6].includes(t.status))

    if (isError) {
        return (
            <div className="flex flex-col items-center justify-center py-24 text-center gap-3">
                <AlertCircle size={40} className="text-muted-foreground/40" />
                <div>
                    <p className="text-sm font-medium text-muted-foreground">Transmission inaccessible</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                        Vérifiez que le proxy nginx est configuré et que Transmission est démarré.
                    </p>
                </div>
                <Button variant="outline" size="sm" asChild>
                    <a href="/transmission" target="_blank" rel="noopener noreferrer">
                        <ExternalLink size={13} className="mr-1.5" /> Ouvrir Transmission
                    </a>
                </Button>
            </div>
        )
    }

    if (isLoading) {
        return (
            <div className="flex items-center justify-center py-16 gap-2 text-muted-foreground">
                <Loader2 size={20} className="animate-spin" />
                <span className="text-sm">Connexion à Transmission…</span>
            </div>
        )
    }

    if (torrents.length === 0) {
        return (
            <div className="flex flex-col items-center justify-center py-24 text-center">
                <Download size={40} className="mb-3 text-muted-foreground/30" />
                <p className="text-sm text-muted-foreground">Aucun téléchargement en cours</p>
                <p className="mt-1 text-xs text-muted-foreground">
                    Ajoutez des torrents via Radarr, Sonarr, ou directement dans Transmission.
                </p>
                <Button variant="outline" size="sm" className="mt-4" asChild>
                    <a href="/transmission" target="_blank" rel="noopener noreferrer">
                        <ExternalLink size={13} className="mr-1.5" /> Ouvrir Transmission
                    </a>
                </Button>
            </div>
        )
    }

    function TorrentSection({ title, items }: { title: string; items: Torrent[] }) {
        if (items.length === 0) return null
        return (
            <div>
                <h2 className="mb-2 text-xs font-medium uppercase tracking-wider text-muted-foreground">{title} ({items.length})</h2>
                <div className="space-y-2">
                    {items.map(t => {
                        const status = TORRENT_STATUS[t.status] ?? { label: 'Inconnu', color: 'text-muted-foreground' }
                        const pct = Math.round(t.percentDone * 100)
                        const hasError = t.error !== 0

                        return (
                            <Card key={t.id} className={cn(hasError && 'border-destructive/40')}>
                                <CardContent className="p-4">
                                    <div className="flex items-start gap-3">
                                        <div className="flex-1 min-w-0">
                                            {/* Name */}
                                            <p className="truncate text-sm font-medium">{t.name}</p>

                                            {/* Error message */}
                                            {hasError && (
                                                <p className="mt-0.5 text-xs text-destructive">{t.errorString}</p>
                                            )}

                                            {/* Progress bar (only for downloading) */}
                                            {t.status === 4 && (
                                                <div className="mt-2">
                                                    <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                                                        <div
                                                            className="h-full rounded-full bg-primary transition-all"
                                                            style={{ width: `${pct}%` }}
                                                        />
                                                    </div>
                                                    <div className="mt-1 flex justify-between text-[10px] text-muted-foreground">
                                                        <span>{pct}% · ↓ {fmtSpeed(t.rateDownload)}</span>
                                                        <span>ETA {fmtEta(t.eta)} · {fmtBytes(t.totalSize)}</span>
                                                    </div>
                                                </div>
                                            )}

                                            {/* Seeding info */}
                                            {t.status === 6 && (
                                                <p className="mt-1 text-[10px] text-muted-foreground">
                                                    ↑ {fmtSpeed(t.rateUpload)} · {fmtBytes(t.totalSize)}
                                                </p>
                                            )}

                                            {/* Stopped info */}
                                            {![4, 6].includes(t.status) && (
                                                <p className="mt-1 text-[10px] text-muted-foreground">
                                                    {fmtBytes(t.totalSize)} · {pct}%
                                                </p>
                                            )}
                                        </div>

                                        {/* Status + actions */}
                                        <div className="flex shrink-0 flex-col items-end gap-2">
                                            <span className={cn('text-xs font-medium', status.color)}>
                                                {status.label}
                                            </span>
                                            {confirmDelete === t.id ? (
                                                <div className="flex gap-1">
                                                    <Button
                                                        size="sm"
                                                        variant="destructive"
                                                        className="h-6 text-[10px] px-2"
                                                        onClick={() => {
                                                            removeTorrent.mutate({ id: t.id, deleteData: true })
                                                            setConfirmDelete(null)
                                                        }}
                                                    >
                                                        Suppr. fichiers
                                                    </Button>
                                                    <Button
                                                        size="sm"
                                                        variant="outline"
                                                        className="h-6 text-[10px] px-2"
                                                        onClick={() => {
                                                            removeTorrent.mutate({ id: t.id, deleteData: false })
                                                            setConfirmDelete(null)
                                                        }}
                                                    >
                                                        Garder fichiers
                                                    </Button>
                                                    <Button
                                                        size="sm"
                                                        variant="ghost"
                                                        className="h-6 text-[10px] px-2"
                                                        onClick={() => setConfirmDelete(null)}
                                                    >
                                                        Annuler
                                                    </Button>
                                                </div>
                                            ) : (
                                                <Button
                                                    size="sm"
                                                    variant="ghost"
                                                    className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                                                    onClick={() => setConfirmDelete(t.id)}
                                                >
                                                    <Trash2 size={13} />
                                                </Button>
                                            )}
                                        </div>
                                    </div>
                                </CardContent>
                            </Card>
                        )
                    })}
                </div>
            </div>
        )
    }

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <p className="text-xs text-muted-foreground">
                    {active.length} actif · {seeding.length} seeding · {stopped.length} arrêté
                </p>
                <Button variant="outline" size="sm" asChild>
                    <a href="/transmission" target="_blank" rel="noopener noreferrer">
                        <ExternalLink size={13} className="mr-1.5" /> Transmission
                    </a>
                </Button>
            </div>
            <TorrentSection title="En cours" items={active} />
            <TorrentSection title="Seeding" items={seeding} />
            <TorrentSection title="Arrêtés" items={stopped} />
        </div>
    )
}

// ── Main page ─────────────────────────────────────────────────────────────────

export function MediaPage() {
    const { active } = useNav()
    // Infer default tab from sidebar nav section
    const defaultTab: MediaTab = active === 'media.downloads' ? 'downloads' : 'movies'
    const [tab, setTab] = useState<MediaTab>(defaultTab)

    return (
        <div className="flex flex-1 flex-col overflow-hidden">
            <PageHeader
                title="Médias"
                subtitle="Bibliothèque Jellyfin · Téléchargements Transmission"
            />

            <div className="flex-1 overflow-y-auto p-6 space-y-4">
                {/* Tab bar */}
                <div className="flex gap-1 rounded-lg bg-muted p-1 w-fit">
                    {TABS.map(t => (
                        <button
                            key={t.id}
                            onClick={() => setTab(t.id)}
                            className={cn(
                                'flex items-center gap-2 rounded-md px-3 py-1.5 text-sm transition-colors',
                                tab === t.id
                                    ? 'bg-background text-foreground shadow-sm font-medium'
                                    : 'text-muted-foreground hover:text-foreground'
                            )}
                        >
                            {t.icon}
                            {t.label}
                            {t.id === 'downloads' && <DownloadBadge />}
                        </button>
                    ))}
                </div>

                {/* Tab content */}
                {tab === 'movies' && <LibraryTab type="movies" />}
                {tab === 'series' && <LibraryTab type="series" />}
                {tab === 'downloads' && <DownloadsTab />}
            </div>
        </div>
    )
}

// Shows active download count on the Downloads tab
function DownloadBadge() {
    const { data } = useTorrents()
    const active = (data?.torrents ?? []).filter((t: Torrent) => t.status === 4).length
    if (!active) return null
    return (
        <span className="flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-medium text-primary-foreground">
            {active}
        </span>
    )
}