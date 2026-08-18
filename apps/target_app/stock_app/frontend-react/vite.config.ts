import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000, // <--- เติมบรรทัดนี้เข้าไปครับ
    strictPort: true, // (ออปชันเสริม) บังคับว่าต้อง 3000 เท่านั้น ถ้าพอร์ตชนให้แจ้งเตือน Error
  }
})
