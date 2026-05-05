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
  LogOut, 
  Search,
  RefreshCw,
  LayoutDashboard,
  Package
} from 'lucide-vue-next'

interface Product {
  id: string
  name: string
  price: number
  stock: number
  created_at: string
}

const products = ref<Product[]>([])
const router = useRouter()
const currentUsername = ref(localStorage.getItem('username') || 'User')
const isLoading = ref(false)
const isSubmitting = ref(false)
const isDialogOpen = ref(false)

const form = reactive({
  name: '',
  displayPrice: '',
  stock: 0
})

const handleLogout = () => {
  localStorage.removeItem('token')
  localStorage.removeItem('user_id')
  localStorage.removeItem('username')
  toast.success('Berhasil logout')
  router.push('/login')
}

const rawPrice = computed(() => {
  return Number(form.displayPrice.replace(/\./g, '')) || 0
})

const handlePriceInput = (e: Event) => {
  const target = e.target as HTMLInputElement
  const clean = target.value.replace(/\D/g, '')
  if (!clean) {
    form.displayPrice = ''
    return
  }
  form.displayPrice = new Intl.NumberFormat('id-ID').format(Number(clean))
}

const fetchProducts = async () => {
  isLoading.value = true
  try {
    const token = localStorage.getItem('token')
    const res = await fetch('/api/products', {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })

    if (res.status === 401) {
      handleLogout()
      return
    }

    const data = await res.json()
    products.value = data || []
  } catch (error) {
    console.error('Fetch error:', error)
    toast.error('Gagal mengambil data produk')
  } finally {
    isLoading.value = false
  }
}

const createProduct = async () => {
  if (!form.name || rawPrice.value <= 0) {
    toast.warning('Mohon isi data dengan lengkap')
    return
  }

  isSubmitting.value = true
  try {
    const token = localStorage.getItem('token')
    const res = await fetch('/api/products', {
      method: 'POST',
      headers: { 
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({
        name: form.name,
        price: rawPrice.value,
        stock: form.stock
      })
    })

    if (res.ok) {
      toast.success('Produk berhasil ditambahkan')
      isDialogOpen.value = false
      form.name = ''
      form.displayPrice = ''
      form.stock = 0
      fetchProducts()
    } else {
      throw new Error('Failed')
    }
  } catch {
    toast.error('Gagal menambah produk')
  } finally {
    isSubmitting.value = false
  }
}

let ws: WebSocket | null = null
const connectWS = () => {
  ws = new WebSocket(`ws://${window.location.host}/ws`)
  
  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      
      // Jika tipe pesan adalah produk baru
      if (data.type === 'PRODUCT_CREATED') {
        const exists = products.value.some(p => p.id === data.id)
        if (!exists) {
          products.value.unshift(data)
          toast.info(`Produk baru: ${data.name}`)
        }
      }
    } catch (e) {
      console.error('WS Error:', e)
    }
  }

  ws.onclose = () => {
    setTimeout(connectWS, 3000)
  }
}

onMounted(() => {
  fetchProducts()
  connectWS()
})

onUnmounted(() => {
  if (ws) ws.close()
})

const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleString('id-ID')
}
</script>

<template>
  <div class="min-h-screen bg-white">
    <!-- Header -->
    <header class="border-b px-8 py-4 flex items-center justify-between sticky top-0 bg-white/80 backdrop-blur-md z-50">
      <div class="flex items-center gap-8">
        <h1 class="text-xl font-bold tracking-tight">Manajemen Produk</h1>
        <nav class="hidden md:flex items-center gap-4 text-sm font-medium">
          <router-link to="/dashboard" class="text-muted-foreground hover:text-foreground flex items-center gap-1.5 transition-colors">
            <LayoutDashboard class="w-4 h-4" />
            Dashboard
          </router-link>
          <router-link to="/products" class="text-foreground flex items-center gap-1.5">
            <Package class="w-4 h-4" />
            Produk
          </router-link>
        </nav>
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
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-4 flex-1">
          <div class="relative w-full max-w-sm">
            <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input placeholder="Cari produk..." class="pl-9 bg-slate-50/50" />
          </div>
          <Button variant="ghost" size="sm" @click="fetchProducts" :disabled="isLoading">
            <RefreshCw class="h-4 w-4 mr-2" :class="isLoading ? 'animate-spin' : ''" />
            Refresh
          </Button>
        </div>

        <Dialog v-model:open="isDialogOpen">
          <DialogTrigger as-child>
            <Button size="sm">
              <Plus class="w-4 h-4 mr-2" />
              Tambah Produk
            </Button>
          </DialogTrigger>
          <DialogContent class="sm:max-w-[425px]">
            <DialogHeader>
              <DialogTitle>Tambah Produk Baru</DialogTitle>
              <DialogDescription>Masukkan detail produk untuk ditambahkan ke database.</DialogDescription>
            </DialogHeader>
            <div class="grid gap-4 py-4">
              <div class="grid gap-2">
                <Label for="name">Nama Produk</Label>
                <Input id="name" v-model="form.name" placeholder="Contoh: Kopi Susu" />
              </div>
              <div class="grid gap-2">
                <Label for="price">Harga (Rp)</Label>
                <Input id="price" v-model="form.displayPrice" @input="handlePriceInput" placeholder="0" />
              </div>
              <div class="grid gap-2">
                <Label for="stock">Stok</Label>
                <Input id="stock" type="number" v-model="form.stock" placeholder="0" />
              </div>
            </div>
            <DialogFooter>
              <Button class="w-full" :disabled="isSubmitting" @click="createProduct">
                <Loader2 v-if="isSubmitting" class="w-4 h-4 mr-2 animate-spin" />
                Simpan Produk
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      <div class="rounded-md border bg-white overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow class="bg-slate-50/50">
              <TableHead>Nama Produk</TableHead>
              <TableHead class="text-right">Harga</TableHead>
              <TableHead class="text-center">Stok</TableHead>
              <TableHead class="text-right">Ditambahkan</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="product in products" :key="product.id" class="hover:bg-slate-50/30 transition-colors">
              <TableCell class="font-semibold text-slate-700">{{ product.name }}</TableCell>
              <TableCell class="text-right font-bold tabular-nums">Rp {{ product.price.toLocaleString('id-ID') }}</TableCell>
              <TableCell class="text-center">
                <span :class="product.stock > 0 ? 'text-green-600' : 'text-red-600'" class="font-medium">
                  {{ product.stock }}
                </span>
              </TableCell>
              <TableCell class="text-right text-muted-foreground text-xs">{{ formatDate(product.created_at) }}</TableCell>
            </TableRow>
            <TableRow v-if="products.length === 0">
              <TableCell colspan="4" class="h-40 text-center text-muted-foreground italic">
                Belum ada data produk.
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </main>
  </div>
</template>
