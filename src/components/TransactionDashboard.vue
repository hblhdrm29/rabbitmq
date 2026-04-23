<script setup lang="ts">
import { ref, onMounted, onUnmounted, reactive, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import { 
  Table, 
  TableBody, 
  TableCell, 
  TableHead, 
  TableHeader, 
  TableRow 
} from '@/components/ui/table'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { 
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { toast } from 'vue-sonner'
import { Plus, Loader2, CheckCircle2, LogOut, User, RefreshCw } from 'lucide-vue-next'

interface Transaction {
  id: string
  amount: number
  description: string
  status: string
  merchant_name: string
  user_id: string
  created_at: string
  payment_method: string
}

const transactions = ref<Transaction[]>([])
const router = useRouter()
const currentUsername = ref(localStorage.getItem('username') || 'User')

const handleLogout = () => {
  localStorage.removeItem('token')
  localStorage.removeItem('user_id')
  localStorage.removeItem('username')
  toast.success('Berhasil logout')
  router.push('/login')
}

const total = ref(0)
const page = ref(1)
const limit = ref(10)
const isLoading = ref(false)
const isSubmitting = ref(false)
const isDialogOpen = ref(false)
const wsStatus = ref<'connecting' | 'connected' | 'disconnected'>('connecting')

const txState = ref<'idle' | 'processing' | 'success'>('idle')
const currentTxId = ref<string | null>(null)

const form = reactive({
  merchant_name: '',
  description: '',
  displayAmount: '',
  payment_method: 'CASH'
})

// Convert display string "10.000" back to number 10000
const rawAmount = computed(() => {
  return Number(form.displayAmount.replace(/\./g, '')) || 0
})

const formatInputAmount = (val: string) => {
  // Remove non-digits
  const clean = val.replace(/\D/g, '')
  if (!clean) return ''
  // Add dots as thousand separators
  return new Intl.NumberFormat('id-ID').format(Number(clean))
}

const handleAmountInput = (e: Event) => {
  const target = e.target as HTMLInputElement
  form.displayAmount = formatInputAmount(target.value)
}

const fetchTransactions = async (p = 1) => {
  isLoading.value = true
  try {
    const token = localStorage.getItem('token')
    const res = await fetch(`http://localhost:8080/transactions?page=${p}&limit=${limit.value}`, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })

    if (res.status === 401) {
      handleLogout()
      return
    }

    const json = await res.json()
    transactions.value = json.data || []
    total.value = json.total || 0
    page.value = json.page || 1
  } catch (error) {
    console.error('Failed to fetch transactions:', error)
    toast.error('Gagal mengambil data transaksi')
  } finally {
    isLoading.value = false
  }
}

const createTransaction = async () => {
  if (!form.merchant_name || rawAmount.value <= 0) {
    toast.warning('Mohon isi data dengan lengkap')
    return
  }

  isSubmitting.value = true
  txState.value = 'processing'

  try {
    const token = localStorage.getItem('token')
    const userId = localStorage.getItem('user_id')

    const res = await fetch('http://localhost:8080/transactions', {
      method: 'POST',
      headers: { 
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({
        merchant_name: form.merchant_name,
        description: form.description,
        amount: rawAmount.value,
        payment_method: form.payment_method,
        user_id: userId
      })
    })

    if (res.status === 401) {
      handleLogout()
      return
    }

    if (res.ok) {
      const newTx = await res.json()
      currentTxId.value = newTx.id
      
      // Inject the transaction into the local list immediately
      if (!newTx.created_at) newTx.created_at = new Date().toISOString()
      if (!newTx.status) newTx.status = 'PENDING'
      
      // Cek apakah data sudah masuk lewat WebSocket (Worker) sebelum respon HTTP selesai
      const exists = transactions.value.some(t => String(t.id).toLowerCase() === String(newTx.id).toLowerCase())
      
      if (!exists && page.value === 1) {
        if (!newTx.created_at) newTx.created_at = new Date().toISOString()
        if (!newTx.status) newTx.status = 'PENDING'
        
        transactions.value.unshift(newTx)
        if (transactions.value.length > limit.value) transactions.value.pop()
        total.value += 1
      }

      // Don't close dialog, leave it as processing
    } else {
      throw new Error('Gagal membuat transaksi')
    }
  } catch (error) {
    console.error('Create transaction error:', error)
    toast.error('Gagal membuat transaksi')
    txState.value = 'idle'
  } finally {
    isSubmitting.value = false
  }
}

const closeDialog = () => {
  isDialogOpen.value = false
}

const preventCloseIfProcessing = (e: Event) => {
  if (txState.value === 'processing' || txState.value === 'success') {
    e.preventDefault()
  }
}

watch(isDialogOpen, (newVal) => {
  if (!newVal) {
    setTimeout(() => {
      txState.value = 'idle'
      currentTxId.value = null
      form.merchant_name = ''
      form.description = ''
      form.displayAmount = ''
      form.payment_method = 'CASH'
    }, 300)
  }
})

// WebSocket setup
let ws: WebSocket | null = null

const connectWS = () => {
  wsStatus.value = 'connecting'
  ws = new WebSocket('ws://localhost:8086/ws')
  
  ws.onopen = () => {
    console.log('WebSocket Connected')
    wsStatus.value = 'connected'
  }

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      console.log('WS Message Received:', data)
      
      // Jika ini adalah event status update (dari Consumer)
      if (data.status === 'SUCCESS' && data.id) {
         // Update status di list lokal agar berubah jadi hijau seketika
         const index = transactions.value.findIndex(t => String(t.id).toLowerCase() === String(data.id).toLowerCase())
         if (index !== -1) {
           const tx = transactions.value[index]
           if (tx) {
             transactions.value[index] = { ...tx, status: 'SUCCESS' }
           }
         }

         // Update UI Modal if it is the current transaction
         if (currentTxId.value && String(currentTxId.value).toLowerCase() === String(data.id).toLowerCase()) {
           txState.value = 'success'
         } else {
           toast.success(`Pembayaran Berhasil!`, {
             description: `ID: ${data.id.substring(0, 8)}... telah sukses diproses.`
           })
         }
         return
      }

      // Jika ini adalah transaksi baru (dari Worker)
      const existingIndex = transactions.value.findIndex(t => String(t.id).toLowerCase() === String(data.id).toLowerCase())
      
      if (existingIndex === -1 && page.value === 1) {
        if (!data.created_at) data.created_at = new Date().toISOString()
        transactions.value.unshift(data)
        if (transactions.value.length > limit.value) transactions.value.pop()
        total.value += 1
        
        // Jangan tampilkan toast "Transaksi Baru" jika ini adalah transaksi yang kita buat sendiri
        const isCurrentTx = currentTxId.value && String(currentTxId.value).toLowerCase() === String(data.id).toLowerCase()
        if (!isCurrentTx) {
          toast.success(`Transaksi Baru!`, {
            description: `${data.merchant_name} - Rp ${data.amount.toLocaleString('id-ID')}`,
          })
        }
      }
    } catch (e) {
      console.error('Error parsing WS message:', e)
    }
  }

  ws.onerror = (err) => {
    console.error('WebSocket Error:', err)
    wsStatus.value = 'disconnected'
  }

  ws.onclose = () => {
    console.log('WebSocket Closed. Retrying in 3s...')
    wsStatus.value = 'disconnected'
    setTimeout(connectWS, 3000)
  }
}

onMounted(() => {
  fetchTransactions()
  connectWS()
})

onUnmounted(() => {
  if (ws) ws.close()
})

const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleString('id-ID')
}

const getStatusVariant = (status: string) => {
  switch (status) {
    case 'SUCCESS': return 'default'
    case 'PENDING': return 'secondary'
    case 'FAILED': return 'destructive'
    default: return 'outline'
  }
}
</script>

<template>
  <div class="p-8 max-w-6xl mx-auto space-y-8 font-sans bg-zinc-50/50 min-h-screen">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div>
        <h1 class="text-3xl font-extrabold tracking-tight text-zinc-900 dark:text-zinc-50 flex items-center gap-3">
          <div class="w-10 h-10 bg-blue-600 rounded-xl flex items-center justify-center text-white shadow-lg shadow-blue-500/20">
            <RefreshCw class="w-6 h-6" />
          </div>
          Real-time Dashboard
        </h1>
        <p class="text-zinc-500 dark:text-zinc-400 mt-1 font-medium flex items-center gap-2">
          Monitor transaksi microservices secara instan
          <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300">
            <span class="w-1.5 h-1.5 rounded-full bg-blue-500 mr-1.5 animate-pulse"></span>
            Live
          </span>
        </p>
      </div>

      <div class="flex items-center gap-3">
        <div class="hidden sm:flex items-center gap-2 px-3 py-1.5 bg-white dark:bg-zinc-800 rounded-full border border-zinc-200 dark:border-zinc-700 shadow-sm">
          <div class="w-6 h-6 bg-zinc-100 dark:bg-zinc-700 rounded-full flex items-center justify-center border border-zinc-200 dark:border-zinc-600">
            <User class="w-3.5 h-3.5 text-zinc-600 dark:text-zinc-400" />
          </div>
          <span class="text-sm font-semibold text-zinc-700 dark:text-zinc-300">{{ currentUsername }}</span>
        </div>
        <Button variant="outline" size="icon" class="rounded-full h-10 w-10 border-zinc-200 dark:border-zinc-800" @click="handleLogout">
          <LogOut class="w-4 h-4 text-zinc-600 dark:text-zinc-400" />
        </Button>
        <div class="w-px h-6 bg-zinc-200 dark:bg-zinc-800 mx-1"></div>
        <Dialog v-model:open="isDialogOpen">
          <DialogTrigger as-child>
            <Button class="gap-2 shadow-lg hover:shadow-xl transition-all duration-300">
              <Plus :size="18" /> Tambah Transaksi
            </Button>
          </DialogTrigger>
          <DialogContent 
            class="sm:max-w-[425px]" 
            @interact-outside="preventCloseIfProcessing"
            @escape-keydown="preventCloseIfProcessing"
          >
            <template v-if="txState === 'idle'">
              <DialogHeader>
                <DialogTitle>Buat Transaksi Baru</DialogTitle>
                <DialogDescription>
                  Masukkan detail transaksi. Data akan diproses melalui RabbitMQ.
                </DialogDescription>
              </DialogHeader>
              <div class="grid gap-6 py-4">
                <div class="space-y-2">
                  <Label for="merchant">Merchant</Label>
                  <Input id="merchant" v-model="form.merchant_name" placeholder="Contoh: Shopee, Tokopedia" />
                </div>
                <div class="space-y-2">
                  <Label for="desc">Deskripsi</Label>
                  <Input id="desc" v-model="form.description" placeholder="Kopi Susu, Token Listrik" />
                </div>
                <div class="space-y-2">
                  <Label for="amount">Jumlah</Label>
                  <div class="relative">
                    <span class="absolute left-3 top-2 text-muted-foreground font-medium">Rp</span>
                    <Input 
                      id="amount" 
                      v-model="form.displayAmount" 
                      @input="handleAmountInput"
                      placeholder="0" 
                      class="pl-10 font-mono font-bold text-lg"
                    />
                  </div>
                </div>
                <div class="space-y-2">
                  <Label>Metode Pembayaran</Label>
                  <div class="flex gap-4">
                    <label class="flex items-center gap-2 cursor-pointer p-3 border rounded-lg flex-1 border-primary bg-primary/5">
                      <input type="radio" v-model="form.payment_method" value="CASH" class="hidden" />
                      <div class="w-4 h-4 rounded-full border border-primary flex items-center justify-center">
                        <div class="w-2 h-2 rounded-full bg-primary" v-if="form.payment_method === 'CASH'"></div>
                      </div>
                      <span class="font-medium text-sm">Tunai (Cash)</span>
                    </label>
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button type="submit" class="w-full" :disabled="isSubmitting" @click="createTransaction">
                  Simpan Transaksi (Rp {{ form.displayAmount || '0' }})
                </Button>
              </DialogFooter>
            </template>
            
            <template v-else-if="txState === 'processing'">
              <div class="flex flex-col items-center justify-center py-12 space-y-6">
                <Loader2 class="h-16 w-16 animate-spin text-primary" />
                <div class="space-y-2 text-center">
                  <h3 class="text-2xl font-bold">Memproses Transaksi</h3>
                  <p class="text-muted-foreground">Data sedang dikirim ke RabbitMQ dan diproses oleh Consumer...</p>
                </div>
              </div>
            </template>
            
            <template v-else-if="txState === 'success'">
              <div class="flex flex-col items-center justify-center py-10 space-y-6">
                <div class="h-24 w-24 bg-green-100 rounded-full flex items-center justify-center text-green-600 mb-2 animate-in zoom-in duration-300">
                  <CheckCircle2 class="h-12 w-12" />
                </div>
                <div class="space-y-2 text-center">
                  <h3 class="text-2xl font-bold text-green-600">Transaksi Berhasil!</h3>
                  <p class="text-muted-foreground">Data telah tersimpan di database dan sinkronisasi real-time selesai.</p>
                </div>
                <Button class="mt-4 w-full" @click="closeDialog">Selesai</Button>
              </div>
            </template>
          </DialogContent>
        </Dialog>

        <div class="text-right">
          <p class="text-sm font-medium text-muted-foreground">Total Transaksi</p>
          <p class="text-2xl font-black text-primary">{{ total.toLocaleString('id-ID') }}</p>
        </div>
      </div>
    </div>

    <Card class="shadow-sm border-none bg-card/50 backdrop-blur-sm">
      <CardHeader>
        <CardTitle>Riwayat Transaksi</CardTitle>
        <CardDescription>Menampilkan transaksi terbaru dari database MySQL.</CardDescription>
      </CardHeader>
      <CardContent>
        <div class="rounded-xl border overflow-hidden bg-background">
          <Table>
            <TableHeader class="bg-muted/50">
              <TableRow>
                <TableHead class="w-[200px]">Waktu</TableHead>
                <TableHead>Merchant</TableHead>
                <TableHead>Deskripsi</TableHead>
                <TableHead>Pembayaran</TableHead>
                <TableHead>Status</TableHead>
                <TableHead class="text-right">Jumlah</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-if="isLoading">
                <TableCell colspan="6" class="h-24 text-center">
                   <div class="flex items-center justify-center gap-2">
                     <Loader2 class="h-5 w-5 animate-spin text-primary" />
                     Memuat data transaksi...
                   </div>
                </TableCell>
              </TableRow>
              <TableRow v-else-if="transactions.length === 0">
                <TableCell colspan="6" class="h-24 text-center">Tidak ada transaksi.</TableCell>
              </TableRow>
              <TableRow v-for="t in transactions" :key="t.id" class="group hover:bg-muted/30 transition-colors">
                <TableCell class="text-xs text-muted-foreground font-medium">{{ formatDate(t.created_at) }}</TableCell>
                <TableCell class="font-bold text-primary">{{ t.merchant_name }}</TableCell>
                <TableCell>{{ t.description }}</TableCell>
                <TableCell>
                  <Badge variant="outline" class="font-bold text-xs">{{ t.payment_method || 'CASH' }}</Badge>
                </TableCell>
                <TableCell>
                  <Badge :variant="getStatusVariant(t.status)" class="font-bold">{{ t.status }}</Badge>
                </TableCell>
                <TableCell class="text-right font-mono font-black text-lg">
                  <span class="text-xs font-normal text-muted-foreground mr-1">Rp</span>
                  {{ t.amount.toLocaleString('id-ID') }}
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <div class="mt-6 flex items-center justify-between">
          <p class="text-sm text-muted-foreground">
            Menampilkan <span class="font-bold text-foreground">{{ transactions.length }}</span> dari <span class="font-bold text-foreground">{{ total.toLocaleString('id-ID') }}</span> transaksi
          </p>
          
          <div class="flex items-center space-x-2">
             <Button 
               variant="outline" 
               size="sm" 
               :disabled="page === 1 || isLoading"
               @click="fetchTransactions(page - 1)"
               class="rounded-lg"
             >
               Previous
             </Button>
             <div class="text-sm font-bold bg-muted px-4 py-1 rounded-lg">Halaman {{ page }}</div>
             <Button 
               variant="outline" 
               size="sm" 
               :disabled="page * limit >= total || isLoading"
               @click="fetchTransactions(page + 1)"
               class="rounded-lg"
             >
               Next
             </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  </div>
</template>
