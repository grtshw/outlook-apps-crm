import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { createGuestListShare } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Copy, Check } from 'lucide-react'

interface ShareDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  listId: string
  listName: string
  eventName: string
}

export function ShareDialog({ open, onOpenChange, listId, listName }: ShareDialogProps) {
  const queryClient = useQueryClient()

  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [shareResult, setShareResult] = useState<{
    id: string
    token: string
    share_url: string
    expires_at: string
  } | null>(null)
  const [copied, setCopied] = useState(false)

  const shareMutation = useMutation({
    mutationFn: () =>
      createGuestListShare(listId, {
        recipient_email: email,
        recipient_name: name || undefined,
      }),
    onSuccess: (data) => {
      setShareResult(data)
      toast.success('Share link created')
    },
    onError: (error: Error) => {
      toast.error(error.message)
    },
  })

  function handleShare() {
    shareMutation.mutate()
  }

  function resetForm() {
    setEmail('')
    setName('')
    setShareResult(null)
    setCopied(false)
    shareMutation.reset()
  }

  function handleClose() {
    resetForm()
    queryClient.invalidateQueries({ queryKey: ['guest-list-shares', listId] })
    onOpenChange(false)
  }

  async function copyURL() {
    if (!shareResult) return
    try {
      await navigator.clipboard.writeText(shareResult.share_url)
      setCopied(true)
      toast.success('Link copied to clipboard')
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error('Failed to copy link')
    }
  }

  const formattedDate = shareResult
    ? new Date(shareResult.expires_at).toLocaleDateString()
    : ''

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Share guest list</DialogTitle>
          <DialogDescription>
            Share "{listName}" with a client for review.
          </DialogDescription>
        </DialogHeader>

        {shareResult ? (
          <div className="space-y-4">
            <p className="text-sm text-muted-foreground">
              Share link created and email notification sent.
            </p>
            <div className="flex gap-2">
              <Input value={shareResult.share_url} readOnly />
              <Button variant="outline" size="icon" onClick={copyURL}>
                {copied ? (
                  <Check className="h-4 w-4" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
              </Button>
            </div>
            <p className="text-sm text-muted-foreground">
              Link expires {formattedDate}
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Recipient email *
              </label>
              <Input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="client@example.com"
                required
              />
            </div>
            <div>
              <label className="block text-sm text-muted-foreground mb-1.5">
                Recipient name
              </label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Jane Smith"
              />
            </div>
          </div>
        )}

        <DialogFooter>
          {shareResult ? (
            <>
              <Button variant="outline" onClick={resetForm}>
                Share with another
              </Button>
              <Button onClick={handleClose}>Done</Button>
            </>
          ) : (
            <>
              <Button variant="outline" onClick={handleClose}>
                Cancel
              </Button>
              <Button
                onClick={handleShare}
                disabled={!email || shareMutation.isPending}
              >
                {shareMutation.isPending ? 'Sharing...' : 'Share'}
              </Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
