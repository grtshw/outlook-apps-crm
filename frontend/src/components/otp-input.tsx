import { useRef, useState, useEffect, type KeyboardEvent, type ClipboardEvent } from 'react'
import { Input } from '@/components/ui/input'

interface OTPInputProps {
  length?: number
  onComplete: (code: string) => void
  disabled?: boolean
}

export function OTPInput({ length = 6, onComplete, disabled = false }: OTPInputProps) {
  const [values, setValues] = useState<string[]>(Array(length).fill(''))
  const inputRefs = useRef<(HTMLInputElement | null)[]>([])

  useEffect(() => {
    inputRefs.current[0]?.focus()
  }, [])

  const handleChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return

    const digit = value.slice(-1)
    const next = [...values]
    next[index] = digit
    setValues(next)

    if (digit && index < length - 1) {
      inputRefs.current[index + 1]?.focus()
    }

    const code = next.join('')
    if (code.length === length && next.every((v) => v !== '')) {
      onComplete(code)
    }
  }

  const handleKeyDown = (index: number, e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Backspace' && !values[index] && index > 0) {
      inputRefs.current[index - 1]?.focus()
    }
  }

  const handlePaste = (e: ClipboardEvent<HTMLInputElement>) => {
    e.preventDefault()
    const pasted = e.clipboardData.getData('text').replace(/\D/g, '').slice(0, length)
    if (!pasted) return

    const next = [...values]
    for (let i = 0; i < pasted.length; i++) {
      next[i] = pasted[i]
    }
    setValues(next)

    const focusIndex = Math.min(pasted.length, length - 1)
    inputRefs.current[focusIndex]?.focus()

    if (pasted.length === length) {
      onComplete(pasted)
    }
  }

  return (
    <div className="flex gap-2 justify-center">
      {values.map((value, index) => (
        <Input
          key={index}
          ref={(el) => { inputRefs.current[index] = el }}
          type="text"
          inputMode="numeric"
          maxLength={1}
          value={value}
          onChange={(e) => handleChange(index, e.target.value)}
          onKeyDown={(e) => handleKeyDown(index, e)}
          onPaste={index === 0 ? handlePaste : undefined}
          disabled={disabled}
          className="w-12 h-14 text-center text-xl"
          autoComplete="one-time-code"
        />
      ))}
    </div>
  )
}
