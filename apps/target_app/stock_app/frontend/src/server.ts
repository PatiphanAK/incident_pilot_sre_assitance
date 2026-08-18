import express from 'express';
import path from 'path';


const app = express();
const PORT = process.env.PORT || 3000;

// เสิร์ฟหน้าเว็บตามปกติ
app.use(express.static(path.join(__dirname, '../public')));

app.listen(PORT, () => {
  console.log(`🚀 Frontend Server เปิดใช้งานแล้วที่ http://localhost:${PORT}`);
});
