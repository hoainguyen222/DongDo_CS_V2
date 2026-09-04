/**
 * ============================================================
 * Automation Test Configuration
 * ============================================================
 * Cấu hình cho tất cả các bài test automation
 */

// Cấu hình server
export const TEST_CONFIG = {
  // Backend API URL
  API_BASE: process.env.TEST_API_BASE || 'http://localhost:8080',
  
  // WebSocket URL
  WS_BASE: process.env.TEST_WS_BASE || 'ws://localhost:8080',
  
  // Số lượng guest tối đa test đồng thời
  MAX_CONCURRENT_GUESTS: parseInt(process.env.MAX_CONCURRENT_GUESTS || '50'),
  
  // Số lượng message tối đa test đồng thời
  MAX_CONCURRENT_MESSAGES: parseInt(process.env.MAX_CONCURRENT_MESSAGES || '100'),
  
  // Timeout cho mỗi request (ms)
  REQUEST_TIMEOUT: parseInt(process.env.REQUEST_TIMEOUT || '30000'),
  
  // Thời gian chờ AI response (ms)
  AI_RESPONSE_TIMEOUT: parseInt(process.env.AI_RESPONSE_TIMEOUT || '60000'),
  
  // Delay giữa các message (ms)
  MESSAGE_DELAY: parseInt(process.env.MESSAGE_DELAY || '100'),
  
  // Số lần retry khi fail
  MAX_RETRIES: parseInt(process.env.MAX_RETRIES || '3'),
  
  // Delay giữa các retry (ms)
  RETRY_DELAY: parseInt(process.env.RETRY_DELAY || '1000'),
};

// Cấu hình test data
export const TEST_DATA = {
  // Danh sách tên guest test
  GUEST_NAMES: [
    'TestGuest001', 'TestGuest002', 'TestGuest003', 'TestGuest004', 'TestGuest005',
    'TestGuest006', 'TestGuest007', 'TestGuest008', 'TestGuest009', 'TestGuest010',
  ],
  
  // Danh sách câu hỏi test cho AI
  AI_TEST_QUESTIONS: [
    'Hàng hóa phái sinh là gì?',
    'Cách nạp tiền vào tài khoản?',
    'Giới thiệu về DDP Invest',
    'Quản lý rủi ro khi giao dịch',
    'Phí giao dịch là bao nhiêu?',
    'Làm sao để rút tiền?',
    'Thời gian giao dịch là khi nào?',
    'Có ứng dụng mobile không?',
  ],
  
  // Câu hỏi cần human CS
  HUMAN_CS_TRIGGERS: [
    'tôi cần hỗ trợ', 
    'không hiểu', 
    'giúp tôi', 
    'cần người thật',
    'chuyên viên',
  ],
};

// Các loại test
export enum TestType {
  GUEST_CONCURRENCY = 'guest_concurrency',
  MESSAGE_CONCURRENCY = 'message_concurrency',
  WEBSOCKET = 'websocket',
  AI_RESPONSE = 'ai_response',
  TEAM_AGENT = 'team_agent',
  ALL = 'all',
}

// Kết quả test
export interface TestResult {
  testType: TestType;
  success: boolean;
  duration: number;
  metrics: {
    total: number;
    success: number;
    failed: number;
    errors: string[];
  };
  details?: any;
}

// Logger
export const testLogger = {
  info: (msg: string, ...args: any[]) => {
    console.log(`[TEST INFO] ${new Date().toISOString()} - ${msg}`, ...args);
  },
  success: (msg: string, ...args: any[]) => {
    console.log(`[TEST SUCCESS] ${new Date().toISOString()} - ${msg}`, ...args);
  },
  error: (msg: string, ...args: any[]) => {
    console.error(`[TEST ERROR] ${new Date().toISOString()} - ${msg}`, ...args);
  },
  warn: (msg: string, ...args: any[]) => {
    console.warn(`[TEST WARN] ${new Date().toISOString()} - ${msg}`, ...args);
  },
};
