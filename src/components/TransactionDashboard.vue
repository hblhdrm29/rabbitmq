<script setup lang="ts">
import { ref, onMounted, onUnmounted, reactive, computed } from 'vue'
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
import { Toaster } from '@/components/ui/sonner'
import { toast } from 'vue-sonner'
import { Plus, Loader2 } from 'lucide-vue-next'

interface Transaction {
  id: string
  amount: number
  description: string
  status: string
  merchant_name: string
  user_id: string
  created_at: string
}

const transactions = ref<Transaction[]>([])
const total = ref(0)
const page = ref(1)
const limit = ref(10)
const isLoading = ref(false)
const isSubmitting = ref(false)
const isDialogOpen = ref(false)

const form = reactive({
  merchant_name: '',
  description: '',
  displayAmount: ''
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
    const res = await fetch(`http://localhost:8080/transactions?page=${p}&limit=${limit.value}`)
    const json = await res.json()
    transactions.value = json.data
    total.value = json.total
    page.value = json.page
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
  try {
    const res = await fetch('http://localhost:8080/transactions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        merchant_name: form.merchant_name,
        description: form.description,
        amount: rawAmount.value
      })
    })

    if (res.ok) {
      toast.info('Transaksi sedang diproses...', {
        description: 'Menunggu konfirmasi dari Outbox Worker & RabbitMQ'
      })
      isDialogOpen.value = false
      // Reset form
      form.merchant_name = ''
      form.description = ''
      form.displayAmount = ''
    } else {
      throw new Error('Gagal membuat transaksi')
    }
  } catch (error) {
    console.error('Create transaction error:', error)
    toast.error('Gagal membuat transaksi')
  } finally {
    isSubmitting.value = false
  }
}

// WebSocket setup
let ws: WebSocket | null = null

const connectWS = () => {
  ws = new WebSocket('ws://localhost:8086/ws')
  
  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    
    // Jika ini adalah event status update (dari Consumer)
    if (data.status === 'SUCCESS' && data.id) {
       // Update status di list lokal agar berubah jadi hijau seketika
       const index = transactions.value.findIndex(t => t.id === data.id)
       const item = transactions.value[index]
       if (item) {
         item.status = 'SUCCESS'
         toast.success(`Pembayaran Berhasil!`, {
           description: `ID: ${data.id.substring(0, 8)}... telah sukses diproses.`
         })
       }
       return
    }

    // Jika ini adalah transaksi baru (dari Worker)
    toast.success(`Transaksi Baru!`, {
      description: `${data.merchant_name} - Rp ${data.amount.toLocaleString('id-ID')}`,
    })
    
    if (page.value === 1) {
      fetchTransactions(1)
    }
  }

  ws.onclose = () => {
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
  <div class="p-8 max-w-6xl mx-auto space-y-8 font-sans">
    <Toaster position="top-right" />
    
    <div class="flex justify-between items-end">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">Transaction Dashboard</h1>
        <p class="text-muted-foreground font-medium">Monitoring 1,000,000 data transaksi secara real-time.</p>
      </div>
      <div class="flex items-end gap-6">
        <Dialog v-model:open="isDialogOpen">
          <DialogTrigger as-child>
            <Button class="gap-2 shadow-lg hover:shadow-xl transition-all duration-300">
              <Plus :size="18" /> Tambah Transaksi
            </Button>
          </DialogTrigger>
          <DialogContent class="sm:max-w-[425px]">
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
            </div>
            <DialogFooter>
              <Button type="submit" class="w-full" :disabled="isSubmitting" @click="createTransaction">
                <Loader2 v-if="isSubmitting" class="mr-2 h-4 w-4 animate-spin" />
                Simpan Transaksi (Rp {{ form.displayAmount || '0' }})
              </Button>
            </DialogFooter>
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
                <TableHead>Status</TableHead>
                <TableHead class="text-right">Jumlah</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-if="isLoading">
                <TableCell colspan="5" class="h-24 text-center">
                   <div class="flex items-center justify-center gap-2">
                     <Loader2 class="h-5 w-5 animate-spin text-primary" />
                     Memuat data transaksi...
                   </div>
                </TableCell>
              </TableRow>
              <TableRow v-else-if="transactions.length === 0">
                <TableCell colspan="5" class="h-24 text-center">Tidak ada transaksi.</TableCell>
              </TableRow>
              <TableRow v-for="t in transactions" :key="t.id" class="group hover:bg-muted/30 transition-colors">
                <TableCell class="text-xs text-muted-foreground font-medium">{{ formatDate(t.created_at) }}</TableCell>
                <TableCell class="font-bold text-primary">{{ t.merchant_name }}</TableCell>
                <TableCell>{{ t.description }}</TableCell>
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
