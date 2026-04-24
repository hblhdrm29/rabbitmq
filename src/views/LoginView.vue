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
  password: ''
})

const handleLogin = async () => {
  if (!form.value.username || !form.value.password) {
    toast.error('Harap isi username dan password')
    return
  }

  isLoading.value = true
  try {
    const res = await fetch('/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value)
    })

    const data = await res.json()
    if (res.ok) {
      localStorage.setItem('token', data.token)
      localStorage.setItem('user_id', data.user_id)
      localStorage.setItem('username', form.value.username)
      
      toast.success('Login Berhasil', {
        description: 'Selamat datang kembali!'
      })
      
      router.push('/dashboard')
    } else {
      toast.error('Login Gagal', {
        description: data.error || 'Username atau password salah'
      })
    }
  } catch (error) {
    console.error('Login error:', error)
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
      <div class="absolute -top-[10%] -left-[10%] w-[40%] h-[40%] rounded-full bg-blue-500 blur-[120px]"></div>
      <div class="absolute -bottom-[10%] -right-[10%] w-[40%] h-[40%] rounded-full bg-indigo-500 blur-[120px]"></div>
    </div>

    <Card class="w-full max-w-md z-10 shadow-xl border-zinc-200/50 dark:border-zinc-800/50 backdrop-blur-sm bg-white/80 dark:bg-zinc-900/80">
      <CardHeader class="space-y-1 text-center">
        <CardTitle class="text-2xl font-bold tracking-tight">Login ke Dashboard</CardTitle>
        <CardDescription>
          Masukkan kredensial Anda untuk mengakses transaksi
        </CardDescription>
      </CardHeader>
      <CardContent class="grid gap-4">
        <div class="grid gap-2">
          <Label for="username">Username</Label>
          <Input 
            id="username" 
            v-model="form.username" 
            type="text" 
            placeholder="admin" 
            @keyup.enter="handleLogin"
          />
        </div>
        <div class="grid gap-2">
          <Label for="password">Password</Label>
          <Input 
            id="password" 
            v-model="form.password" 
            type="password" 
            @keyup.enter="handleLogin"
          />
        </div>
      </CardContent>
      <CardFooter class="flex flex-col gap-4">
        <Button 
          class="w-full font-semibold" 
          :disabled="isLoading"
          @click="handleLogin"
        >
          <Loader2 v-if="isLoading" class="mr-2 h-4 w-4 animate-spin" />
          {{ isLoading ? 'Memproses...' : 'Login Sekarang' }}
        </Button>
        <div class="text-sm text-center text-zinc-500">
          Belum punya akun? 
          <router-link to="/register" class="text-blue-600 hover:underline font-medium">Daftar di sini</router-link>
        </div>
      </CardFooter>
    </Card>
  </div>
</template>
