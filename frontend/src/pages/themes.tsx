import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { getThemes, createTheme, updateTheme, deleteTheme } from '@/lib/api'
import type { Theme } from '@/lib/pocketbase'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetFooter, SheetSection } from '@/components/ui/sheet'
import { PageHeader } from '@/components/ui/page-header'
import { Plus, Trash2, Palette } from 'lucide-react'

function ColorSwatch({ color, size = 'md' }: { color: string; size?: 'sm' | 'md' }) {
  const dim = size === 'sm' ? 'w-5 h-5' : 'w-8 h-8'
  return (
    <div
      className={`${dim} rounded border border-border shrink-0`}
      style={{ backgroundColor: color }}
    />
  )
}

export default function ThemesPage() {
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)
  const [editingTheme, setEditingTheme] = useState<Theme | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null)

  // Form state
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [colorPrimary, setColorPrimary] = useState('#E95139')
  const [colorSecondary, setColorSecondary] = useState('#645C49')
  const [colorBackground, setColorBackground] = useState('#020202')
  const [colorSurface, setColorSurface] = useState('#1A1917')
  const [colorText, setColorText] = useState('#ffffff')
  const [colorTextMuted, setColorTextMuted] = useState('#A8A9B1')
  const [colorBorder, setColorBorder] = useState('#645C49')
  const [logoUrl, setLogoUrl] = useState('')
  const [logoLightUrl, setLogoLightUrl] = useState('')
  const [heroImageUrl, setHeroImageUrl] = useState('')
  const [isDark, setIsDark] = useState(true)

  const { data: themesData, isLoading } = useQuery({
    queryKey: ['themes'],
    queryFn: getThemes,
  })

  const themes = themesData?.items ?? []

  const createMutation = useMutation({
    mutationFn: (data: Partial<Theme>) => createTheme(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['themes'] })
      toast.success('Theme created')
      setEditOpen(false)
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to create theme'),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<Theme> }) => updateTheme(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['themes'] })
      toast.success('Theme updated')
      setEditOpen(false)
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to update theme'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteTheme(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['themes'] })
      toast.success('Theme deleted')
      setDeleteConfirm(null)
    },
    onError: (err) => toast.error(err instanceof Error ? err.message : 'Failed to delete theme'),
  })

  const openCreate = () => {
    setEditingTheme(null)
    setName('')
    setSlug('')
    setColorPrimary('#E95139')
    setColorSecondary('#645C49')
    setColorBackground('#020202')
    setColorSurface('#1A1917')
    setColorText('#ffffff')
    setColorTextMuted('#A8A9B1')
    setColorBorder('#645C49')
    setLogoUrl('')
    setLogoLightUrl('')
    setHeroImageUrl('')
    setIsDark(true)
    setEditOpen(true)
  }

  const openEdit = (theme: Theme) => {
    setEditingTheme(theme)
    setName(theme.name)
    setSlug(theme.slug)
    setColorPrimary(theme.color_primary)
    setColorSecondary(theme.color_secondary)
    setColorBackground(theme.color_background)
    setColorSurface(theme.color_surface)
    setColorText(theme.color_text)
    setColorTextMuted(theme.color_text_muted)
    setColorBorder(theme.color_border)
    setLogoUrl(theme.logo_url)
    setLogoLightUrl(theme.logo_light_url)
    setHeroImageUrl(theme.hero_image_url)
    setIsDark(theme.is_dark)
    setEditOpen(true)
  }

  const handleSave = () => {
    const data: Partial<Theme> = {
      name,
      slug,
      color_primary: colorPrimary,
      color_secondary: colorSecondary,
      color_background: colorBackground,
      color_surface: colorSurface,
      color_text: colorText,
      color_text_muted: colorTextMuted,
      color_border: colorBorder,
      logo_url: logoUrl,
      logo_light_url: logoLightUrl,
      hero_image_url: heroImageUrl,
      is_dark: isDark,
    }

    if (editingTheme) {
      updateMutation.mutate({ id: editingTheme.id, data })
    } else {
      createMutation.mutate(data)
    }
  }

  const isPending = createMutation.isPending || updateMutation.isPending

  return (
    <>
      <PageHeader title="Themes">
        <Button onClick={openCreate}>
          <Plus className="w-4 h-4 mr-1.5" />
          New theme
        </Button>
      </PageHeader>

      <div className="p-6">
        {isLoading ? (
          <p className="text-sm text-muted-foreground text-center py-12">Loading themes...</p>
        ) : themes.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-12">No themes yet.</p>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {themes.map((theme) => (
              <button
                key={theme.id}
                type="button"
                onClick={() => openEdit(theme)}
                className="border border-border rounded-lg p-5 text-left hover:border-foreground/30 transition-colors cursor-pointer"
              >
                <div className="flex items-center gap-3 mb-3">
                  <Palette className="w-4 h-4 text-muted-foreground" />
                  <span className="text-sm">{theme.name}</span>
                  <span className="text-xs text-muted-foreground ml-auto">{theme.is_dark ? 'Dark' : 'Light'}</span>
                </div>
                <div className="flex gap-1.5">
                  <ColorSwatch color={theme.color_primary} size="sm" />
                  <ColorSwatch color={theme.color_secondary} size="sm" />
                  <ColorSwatch color={theme.color_background} size="sm" />
                  <ColorSwatch color={theme.color_surface} size="sm" />
                  <ColorSwatch color={theme.color_text} size="sm" />
                  <ColorSwatch color={theme.color_border} size="sm" />
                </div>
                {theme.logo_url && (
                  <div className="mt-3 h-6">
                    <img src={theme.logo_url} alt="" className="h-full object-contain" />
                  </div>
                )}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Edit / Create sheet */}
      <Sheet open={editOpen} onOpenChange={(o) => !o && setEditOpen(false)}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{editingTheme ? 'Edit theme' : 'New theme'}</SheetTitle>
          </SheetHeader>
          <div className="flex-1 overflow-y-auto p-6 space-y-6">
            <SheetSection title="Details">
              <div className="space-y-3">
                <div>
                  <label className="block text-sm mb-1.5">Name</label>
                  <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. After Dark" />
                </div>
                <div>
                  <label className="block text-sm mb-1.5">Slug</label>
                  <Input value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="e.g. after-dark" />
                </div>
                <div className="flex items-center justify-between">
                  <label className="text-sm">Dark theme</label>
                  <Switch checked={isDark} onCheckedChange={setIsDark} />
                </div>
              </div>
            </SheetSection>

            <SheetSection title="Colors">
              <div className="space-y-3">
                {[
                  { label: 'Primary', value: colorPrimary, set: setColorPrimary },
                  { label: 'Secondary', value: colorSecondary, set: setColorSecondary },
                  { label: 'Background', value: colorBackground, set: setColorBackground },
                  { label: 'Surface', value: colorSurface, set: setColorSurface },
                  { label: 'Text', value: colorText, set: setColorText },
                  { label: 'Text muted', value: colorTextMuted, set: setColorTextMuted },
                  { label: 'Border', value: colorBorder, set: setColorBorder },
                ].map(({ label, value, set }) => (
                  <div key={label} className="flex items-center gap-3">
                    <input
                      type="color"
                      value={value}
                      onChange={(e) => set(e.target.value)}
                      className="w-8 h-8 rounded border border-border cursor-pointer shrink-0"
                    />
                    <div className="flex-1 min-w-0">
                      <label className="block text-sm mb-0.5">{label}</label>
                      <Input
                        value={value}
                        onChange={(e) => set(e.target.value)}
                        placeholder="#000000"
                        className="h-8 text-xs font-mono"
                      />
                    </div>
                  </div>
                ))}
              </div>
            </SheetSection>

            <SheetSection title="Preview">
              <div
                className="rounded-lg p-6 border border-border overflow-hidden"
                style={{ backgroundColor: colorBackground }}
              >
                <div className="space-y-2">
                  <p style={{ color: colorText, fontFamily: 'PP Museum, Georgia, serif', fontSize: '1.25rem' }}>
                    Heading text
                  </p>
                  <p style={{ color: colorTextMuted, fontSize: '0.875rem' }}>
                    Muted description text
                  </p>
                  <div className="flex gap-2 mt-3">
                    <div
                      className="px-4 py-2 rounded text-sm text-white"
                      style={{ backgroundColor: colorPrimary }}
                    >
                      Button
                    </div>
                  </div>
                  <div
                    className="mt-3 rounded p-3"
                    style={{ backgroundColor: colorSurface, borderColor: colorBorder, borderWidth: 1 }}
                  >
                    <p style={{ color: colorText, fontSize: '0.75rem' }}>Surface card</p>
                  </div>
                </div>
              </div>
            </SheetSection>

            <SheetSection title="Logos">
              <div className="space-y-3">
                <div>
                  <label className="block text-sm mb-1.5">Logo URL</label>
                  <Input value={logoUrl} onChange={(e) => setLogoUrl(e.target.value)} placeholder="/images/logo.svg" />
                  {logoUrl && <img src={logoUrl} alt="" className="h-6 mt-2 object-contain" />}
                </div>
                <div>
                  <label className="block text-sm mb-1.5">Logo (light variant)</label>
                  <Input value={logoLightUrl} onChange={(e) => setLogoLightUrl(e.target.value)} placeholder="/images/logo-white.svg" />
                  {logoLightUrl && (
                    <div className="mt-2 bg-gray-900 rounded p-2 inline-block">
                      <img src={logoLightUrl} alt="" className="h-6 object-contain" />
                    </div>
                  )}
                </div>
              </div>
            </SheetSection>

            <SheetSection title="Hero image">
              <div>
                <label className="block text-sm mb-1.5">Default hero image URL</label>
                <Input value={heroImageUrl} onChange={(e) => setHeroImageUrl(e.target.value)} placeholder="/images/rsvp-hero.jpg" />
                {heroImageUrl && (
                  <img src={heroImageUrl} alt="" className="mt-2 rounded w-full h-32 object-cover" />
                )}
              </div>
            </SheetSection>

            {editingTheme && (
              <div className="pt-4 border-t border-border">
                {deleteConfirm === editingTheme.id ? (
                  <div className="space-y-2">
                    <p className="text-sm text-muted-foreground">Are you sure? This cannot be undone.</p>
                    <div className="flex gap-2">
                      <Button
                        variant="destructive"
                        size="sm"
                        onClick={() => deleteMutation.mutate(editingTheme.id)}
                        disabled={deleteMutation.isPending}
                      >
                        {deleteMutation.isPending ? 'Deleting...' : 'Confirm delete'}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setDeleteConfirm(null)}
                      >
                        Cancel
                      </Button>
                    </div>
                  </div>
                ) : (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="text-muted-foreground"
                    onClick={() => setDeleteConfirm(editingTheme.id)}
                  >
                    <Trash2 className="w-3.5 h-3.5 mr-1.5" />
                    Delete theme
                  </Button>
                )}
              </div>
            )}
          </div>
          <SheetFooter>
            <Button onClick={handleSave} disabled={isPending || !name.trim() || !slug.trim()}>
              {isPending ? 'Saving...' : editingTheme ? 'Save changes' : 'Create theme'}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
