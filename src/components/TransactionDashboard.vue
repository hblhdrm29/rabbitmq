<script setup lang="ts">
import { ref, onMounted, onUnmounted, reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { 
  Table, 
  TableBody, 
  TableCell, 
  TableHead, 
  TableHeader, 
  TableRow 
} from '@/components/ui/table'
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
import { 
  Plus, 
  Loader2, 
  CheckCircle2, 
  LogOut, 
  Search,
  RefreshCw
} from 'lucide-vue-next'

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

const rawAmount = computed(() => {
  return Number(form.displayAmount.replace(/\./g, '')) || 0
})

const handleAmountInput = (e: Event) => {
  const target = e.target as HTMLInputElement
  const clean = target.value.replace(/\D/g, '')
  if (!clean) {
    form.displayAmount = ''
    return
  }
  form.displayAmount = new Intl.NumberFormat('id-ID').format(Number(clean))
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
    console.error('Fetch error:', error)
    toast.error('Gagal mengambil data')
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
    } else {
      throw new Error('Failed')
    }
  } catch (error) {
    console.error('Create error:', error)
    toast.error('Gagal membuat transaksi')
    txState.value = 'idle'
  } finally {
    isSubmitting.value = false
  }
}

let ws: WebSocket | null = null
const connectWS = () => {
  wsStatus.value = 'connecting'
  ws = new WebSocket('ws://localhost:8086/ws')
  
  ws.onopen = () => { wsStatus.value = 'connected' }
  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      if (data.status === 'SUCCESS' && data.id) {
         const idx = transactions.value.findIndex(t => String(t.id).toLowerCase() === String(data.id).toLowerCase())
         if (idx !== -1) {
           const currentTx = transactions.value[idx]
           if (currentTx) transactions.value[idx] = { ...currentTx, status: 'SUCCESS' }
         }
         if (currentTxId.value && String(currentTxId.value).toLowerCase() === String(data.id).toLowerCase()) {
           txState.value = 'success'
         }
      } else {
        const exists = transactions.value.some(t => String(t.id).toLowerCase() === String(data.id).toLowerCase())
        if (!exists && page.value === 1) {
          transactions.value.unshift(data)
          if (transactions.value.length > limit.value) transactions.value.pop()
          total.value += 1
        }
      }
    } catch (e) { console.error(e) }
  }
  ws.onclose = () => { wsStatus.value = 'disconnected'; setTimeout(connectWS, 3000) }
  ws.onerror = () => { wsStatus.value = 'disconnected' }
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
  <div class="min-h-screen bg-white">
    <!-- Clean Top Header -->
    <header class="border-b px-8 py-4 flex items-center justify-between sticky top-0 bg-white/80 backdrop-blur-md z-50">
      <div class="flex items-center gap-4">
        <h1 class="text-xl font-bold tracking-tight">Dashboard</h1>
        <Badge variant="outline" class="gap-1.5 px-2">
          <div class="w-1.5 h-1.5 rounded-full" :class="wsStatus === 'connected' ? 'bg-green-500' : 'bg-red-500'"></div>
          {{ wsStatus === 'connected' ? 'Live' : 'Offline' }}
        </Badge>
      </div>

      <div class="flex items-center gap-6">
        <span class="text-sm font-medium text-muted-foreground">
          Hello, <span class="text-foreground font-bold">{{ currentUsername }}</span>
        </span>
        <Button variant="ghost" size="sm" class="text-muted-foreground hover:text-red-600" @click="handleLogout">
          <LogOut class="w-4 h-4 mr-2" />
          Logout
        </Button>
      </div>
    </header>

    <main class="p-8 max-w-[1600px] mx-auto space-y-8">
      <!-- Toolbar Row -->
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4 flex-1">
          <div class="relative w-full max-w-sm">
            <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input placeholder="Cari transaksi..." class="pl-9 bg-slate-50/50" />
          </div>
          <Button variant="ghost" size="sm" @click="fetchTransactions(page)" :disabled="isLoading">
            <RefreshCw class="h-4 w-4 mr-2" :class="isLoading ? 'animate-spin' : ''" />
            Refresh
          </Button>
        </div>

        <Dialog v-model:open="isDialogOpen">
          <DialogTrigger as-child>
            <Button size="sm">
              <Plus class="w-4 h-4 mr-2" />
              Transaksi Baru
            </Button>
          </DialogTrigger>
          <DialogContent class="sm:max-w-[425px]">
            <template v-if="txState === 'idle'">
              <DialogHeader>
                <DialogTitle>Buat Transaksi</DialogTitle>
                <DialogDescription>Masukkan detail transaksi secara manual.</DialogDescription>
              </DialogHeader>
              <div class="grid gap-4 py-4">
                <div class="grid gap-2">
                  <Label for="merchant">Merchant</Label>
                  <Input id="merchant" v-model="form.merchant_name" placeholder="Tokopedia, Shopee..." />
                </div>
                <div class="grid gap-2">
                  <Label for="amount">Jumlah (Rp)</Label>
                  <Input id="amount" v-model="form.displayAmount" @input="handleAmountInput" placeholder="0" class="font-bold text-lg" />
                </div>
                <div class="grid gap-2">
                  <Label for="method">Metode</Label>
                  <select v-model="form.payment_method" class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm focus:ring-2 focus:ring-ring">
                    <option value="CASH">Cash</option>
                    <option value="QRIS">QRIS</option>
                    <option value="DEBIT">Debit Card</option>
                  </select>
                </div>
              </div>
              <DialogFooter>
                <Button class="w-full" :disabled="isSubmitting" @click="createTransaction">
                  <Loader2 v-if="isSubmitting" class="w-4 h-4 mr-2 animate-spin" />
                  Konfirmasi
                </Button>
              </DialogFooter>
            </template>
            <div v-else-if="txState === 'processing'" class="py-12 text-center space-y-4">
              <Loader2 class="w-10 h-10 animate-spin mx-auto text-primary" />
              <p class="font-medium">Sedang memproses...</p>
            </div>
            <div v-else-if="txState === 'success'" class="py-12 text-center space-y-4">
              <div class="w-16 h-16 bg-green-50 text-green-500 rounded-full flex items-center justify-center mx-auto">
                <CheckCircle2 class="w-8 h-8" />
              </div>
              <h3 class="text-xl font-bold">Berhasil!</h3>
              <Button variant="outline" class="w-full mt-4" @click="isDialogOpen = false">Tutup</Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      <!-- Wide Table -->
      <div class="rounded-md border bg-white">
        <Table>
          <TableHeader>
            <TableRow class="bg-slate-50/50">
              <TableHead class="w-[180px]">Waktu</TableHead>
              <TableHead>Merchant</TableHead>
              <TableHead>Metode</TableHead>
              <TableHead class="text-right">Jumlah</TableHead>
              <TableHead class="text-center">Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="tx in transactions" :key="tx.id" class="hover:bg-slate-50/30 transition-colors">
              <TableCell class="text-muted-foreground tabular-nums text-xs font-medium">{{ formatDate(tx.created_at) }}</TableCell>
              <TableCell class="font-semibold text-slate-700">{{ tx.merchant_name }}</TableCell>
              <TableCell>
                <Badge variant="secondary" class="font-medium text-[10px] uppercase tracking-wider">{{ tx.payment_method }}</Badge>
              </TableCell>
              <TableCell class="text-right font-bold tabular-nums">Rp {{ tx.amount.toLocaleString('id-ID') }}</TableCell>
              <TableCell class="text-center">
                <Badge :variant="getStatusVariant(tx.status)" class="min-w-[80px] justify-center">{{ tx.status }}</Badge>
              </TableCell>
            </TableRow>
            <TableRow v-if="transactions.length === 0">
              <TableCell colspan="5" class="h-40 text-center text-muted-foreground italic">Belum ada data transaksi.</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>

      <!-- Footer Pagination -->
      <div class="flex items-center justify-between px-2">
        <p class="text-sm text-muted-foreground font-medium">
          Menampilkan {{ transactions.length }} dari {{ total }} data
        </p>
        <div class="flex items-center gap-2">
          <Button variant="outline" size="sm" @click="fetchTransactions(page - 1)" :disabled="page <= 1">Previous</Button>
          <div class="text-xs font-bold text-muted-foreground px-4">
            {{ page }} / {{ Math.ceil(total/limit) || 1 }}
          </div>
          <Button variant="outline" size="sm" @click="fetchTransactions(page + 1)" :disabled="page * limit >= total">Next</Button>
        </div>
      </div>
    </main>
  </div>
</template>
