import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { api } from '@/lib/api'

interface ScanFormProps {
  onScanStarted: () => void
}

export function ScanForm({ onScanStarted }: ScanFormProps) {
  const [host, setHost] = useState('192.168.1.200')
  const [port, setPort] = useState(502)
  const [unitId, setUnitId] = useState(1)
  const [addrStart, setAddrStart] = useState(0)
  const [addrEnd, setAddrEnd] = useState(9999)
  const [samples, setSamples] = useState(5)
  const [scanTypes, setScanTypes] = useState({
    holding: true,
    input: true,
    coil: false,
    discrete: false,
  })
  const [loading, setLoading] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const toggleType = (type: keyof typeof scanTypes) => {
    setScanTypes(prev => ({ ...prev, [type]: !prev[type] }))
  }

  const selectedTypes = Object.entries(scanTypes)
    .filter(([, v]) => v)
    .map(([k]) => k)

  const doScan = async (mode: 'full' | 'quick') => {
    setError(null)
    setLoading(mode)
    try {
      const req = {
        host,
        port,
        unit_id: unitId,
        scan_types: selectedTypes,
        address_start: addrStart,
        address_end: addrEnd,
        samples,
      }
      if (mode === 'quick') {
        await api.quickScan(req)
      } else {
        await api.scan(req)
      }
      onScanStarted()
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Scan failed')
    } finally {
      setLoading(null)
    }
  }

  return (
    <Card className="bg-card border-border">
      <CardHeader className="pb-3">
        <CardTitle className="text-sm font-medium text-muted-foreground uppercase tracking-wider">
          Scan Config
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-3 gap-3">
          <div>
            <Label className="text-xs text-muted-foreground">Host</Label>
            <Input
              value={host}
              onChange={e => setHost(e.target.value)}
              placeholder="192.168.1.200"
              className="bg-background font-mono text-sm"
            />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">Port</Label>
            <Input
              type="number"
              value={port}
              onChange={e => setPort(Number(e.target.value))}
              className="bg-background font-mono text-sm"
            />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">Unit ID</Label>
            <Input
              type="number"
              value={unitId}
              onChange={e => setUnitId(Number(e.target.value))}
              min={1}
              max={247}
              className="bg-background font-mono text-sm"
            />
          </div>
        </div>

        <div>
          <Label className="text-xs text-muted-foreground">Register Types</Label>
          <div className="flex gap-2 mt-1">
            {(['holding', 'input', 'coil', 'discrete'] as const).map(type => (
              <button
                key={type}
                onClick={() => toggleType(type)}
                className={`px-3 py-1 rounded text-xs font-mono transition-colors ${
                  scanTypes[type]
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-secondary text-muted-foreground hover:text-foreground'
                }`}
              >
                {type}
              </button>
            ))}
          </div>
        </div>

        <div className="grid grid-cols-3 gap-3">
          <div>
            <Label className="text-xs text-muted-foreground">Start Address</Label>
            <Input
              type="number"
              value={addrStart}
              onChange={e => setAddrStart(Number(e.target.value))}
              min={0}
              className="bg-background font-mono text-sm"
            />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">End Address</Label>
            <Input
              type="number"
              value={addrEnd}
              onChange={e => setAddrEnd(Number(e.target.value))}
              max={65535}
              className="bg-background font-mono text-sm"
            />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">Samples</Label>
            <Input
              type="number"
              value={samples}
              onChange={e => setSamples(Number(e.target.value))}
              min={1}
              max={20}
              className="bg-background font-mono text-sm"
            />
          </div>
        </div>

        {error && (
          <div className="text-sm text-destructive bg-destructive/10 px-3 py-2 rounded">
            {error}
          </div>
        )}

        <div className="flex gap-2">
          <Button
            onClick={() => doScan('quick')}
            disabled={!!loading || !host}
            variant="secondary"
            className="flex-1 font-mono text-xs"
          >
            {loading === 'quick' ? 'Scanning...' : 'Quick Scan'}
          </Button>
          <Button
            onClick={() => doScan('full')}
            disabled={!!loading || !host || selectedTypes.length === 0}
            className="flex-1 font-mono text-xs"
          >
            {loading === 'full' ? 'Scanning...' : 'Full Scan'}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
