<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Loader2 } from 'lucide-vue-next'

const router = useRouter()
const isLoading = ref(false)
const form = ref({
  username: '',
  email: '',
  password: '',
  confirmPassword: ''
})

const handleRegister = async () => {
  if (!form.value.username || !form.value.password) {
    toast.error('Harap isi username dan password')
    return
  }

  if (form.value.password !== form.value.confirmPassword) {
    toast.error('Konfirmasi password tidak cocok')
    return
  }

  isLoading.value = true
  try {
    const res = await fetch('http://localhost:8089/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: form.value.username,
        email: form.value.email,
        password: form.value.password
      })
    })

    const data = await res.json()
    if (res.ok) {
      toast.success('Registrasi Berhasil', {
        description: 'Silakan login dengan akun baru Anda'
      })
      router.push('/login')
    } else {
      toast.error('Registrasi Gagal', {
        description: data.error || 'Gagal membuat akun'
      })
    }
  } catch (error) {
    console.error('Register error:', error)
    toast.error('Server Error', {
      description: 'Gagal terhubung ke Auth Service'
    })
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-zinc-50 dark:bg-zinc-950 p-4">
    <div class="absolute inset-0 z-0 opacity-10 pointer-events-none overflow-hidden">
      <div class="absolute -top-[10%] -right-[10%] w-[40%] h-[40%] rounded-full bg-emerald-500 blur-[120px]"></div>
      <div class="absolute -bottom-[10%] -left-[10%] w-[40%] h-[40%] rounded-full bg-teal-500 blur-[120px]"></div>
    </div>

    <Card class="w-full max-w-md z-10 shadow-xl border-zinc-200/50 dark:border-zinc-800/50 backdrop-blur-sm bg-white/80 dark:bg-zinc-900/80">
      <CardHeader class="space-y-1 text-center">
        <CardTitle class="text-2xl font-bold tracking-tight">Daftar Akun Baru</CardTitle>
        <CardDescription>
          Buat akun untuk mulai mengelola transaksi
        </CardDescription>
      </CardHeader>
      <CardContent class="grid gap-3">
        <div class="grid gap-1.5">
          <Label for="username">Username</Label>
          <Input id="username" v-model="form.username" type="text" placeholder="johndoe" />
        </div>
        <div class="grid gap-1.5">
          <Label for="email">Email (Opsional)</Label>
          <Input id="email" v-model="form.email" type="email" placeholder="name@example.com" />
        </div>
        <div class="grid gap-1.5">
          <Label for="password">Password</Label>
          <Input id="password" v-model="form.password" type="password" />
        </div>
        <div class="grid gap-1.5">
          <Label for="confirmPassword">Konfirmasi Password</Label>
          <Input id="confirmPassword" v-model="form.confirmPassword" type="password" />
        </div>
      </CardContent>
      <CardFooter class="flex flex-col gap-4">
        <Button 
          class="w-full font-semibold bg-emerald-600 hover:bg-emerald-700 dark:bg-emerald-700 dark:hover:bg-emerald-800" 
          :disabled="isLoading"
          @click="handleRegister"
        >
          <Loader2 v-if="isLoading" class="mr-2 h-4 w-4 animate-spin" />
          {{ isLoading ? 'Mendaftar...' : 'Buat Akun' }}
        </Button>
        <div class="text-sm text-center text-zinc-500">
          Sudah punya akun? 
          <router-link to="/login" class="text-blue-600 hover:underline font-medium">Login di sini</router-link>
        </div>
      </CardFooter>
    </Card>
  </div>
</template>
