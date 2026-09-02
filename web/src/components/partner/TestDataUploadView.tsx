'use client';

import React, { useState, useEffect } from 'react';
import './PartnerStyles.css';

interface TestDataUploadViewProps {
  onReportError?: (source: string, title: string, details: string, severity?: 'low' | 'medium' | 'high') => void;
}

export interface TestRowItem {
  session_id: string;
  customer_name: string;
  customer_phone?: string;
  channel: string; // CHAT or CALL
  status: string; // RESOLVED, AI_ACTIVE, NEEDS_HUMAN_CS, HUMAN_CS_ACTIVE
  assigned_cs?: string;
  created_at: string;
  rating?: number;
  issue_category?: string;
  last_message?: string;
}

export const TestDataUploadView: React.FC<TestDataUploadViewProps> = ({ onReportError }) => {
  const [isTestActive, setIsTestActive] = useState(false);
  const [testRows, setTestRows] = useState<TestRowItem[]>([]);
  const [fileError, setFileError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [selectedFileName, setSelectedFileName] = useState('');

  useEffect(() => {
    const active = sessionStorage.getItem('DD_TEST_REPORT_ACTIVE') === 'true';
    setIsTestActive(active);
    const stored = sessionStorage.getItem('DD_TEST_REPORT_DATA');
    if (stored) {
      try {
        setTestRows(JSON.parse(stored));
      } catch (e) {}
    }
  }, []);

  // Download Sample Form CSV Template
  const handleDownloadSample = () => {
    const sampleHeaders = 'session_id,customer_name,customer_phone,channel,status,assigned_cs,created_at,rating,issue_category,last_message\n';
    const sampleRows =
      'SESS-1001,Nguyễn Văn An,0912345678,CHAT,RESOLVED,cskh_01,2026-09-02 09:30:00,5,Quy trình Nạp / Rút tiền DDP Invest,Tài khoản nạp tiền thành công\n' +
      'SESS-1002,Trần Thị Bình,0987654321,CHAT,AI_ACTIVE,,2026-09-02 10:15:00,4,Biểu phí giao dịch Hàng hóa CBOT,Đã tư vấn biểu phí cho khách\n' +
      'SESS-1003,Phạm Minh Cường,0933445566,CALL,NEEDS_HUMAN_CS,admin,2026-09-02 11:00:00,3,Margin Call & Quản trị rủi ro,Khách yêu cầu chuyển gặp quản lý\n' +
      'SESS-1004,Lê Thu Hà,0977889900,CHAT,RESOLVED,cskh_01,2026-09-02 14:20:00,5,Hướng dẫn eKYC mở tài khoản,Xác thực eKYC thành công\n' +
      'SESS-1005,Hoàng Quốc Việt,0909112233,CALL,HUMAN_CS_ACTIVE,admin,2026-09-02 15:05:00,2,Thắc mắc lỗi kỹ thuật app DDP,Đang kiểm tra lỗi kết nối app\n';

    const csvContent = 'data:text/csv;charset=utf-8,\uFEFF' + sampleHeaders + sampleRows;
    const encodedUri = encodeURI(csvContent);
    const link = document.createElement('a');
    link.setAttribute('href', encodedUri);
    link.setAttribute('download', 'DongDo_TestData_Template.csv');
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  // Parse CSV Content & Validate Mandatory Fields with Auto-Detect Delimiter & BOM Stripping
  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFileError(null);
    setSuccessMsg(null);
    const file = e.target.files?.[0];
    if (!file) return;

    setSelectedFileName(file.name);
    const reader = new FileReader();
    reader.onload = (event) => {
      try {
        let rawText = event.target?.result as string;
        if (!rawText) throw new Error('Không thể đọc dữ liệu file.');

        // Clean UTF-8 BOM characters
        rawText = rawText.replace(/^\uFEFF/, '').replace(/^\uFFFE/, '');

        const lines = rawText.split(/\r?\n/).filter((line) => line.trim() !== '');

        if (lines.length <= 1) {
          throw new Error('File rỗng hoặc chỉ có dòng tiêu đề.');
        }

        // Auto-detect delimiter (comma, semicolon, tab, pipe)
        const headerLine = lines[0];
        let delimiter = ',';
        if (headerLine.includes(';')) delimiter = ';';
        else if (headerLine.includes('\t')) delimiter = '\t';
        else if (headerLine.includes('|')) delimiter = '|';

        // Parse headers cleanly
        const headers = headerLine
          .split(delimiter)
          .map((h) => h.replace(/["'\r\n]/g, '').trim().toLowerCase());

        const requiredFields = ['session_id', 'customer_name', 'channel', 'status', 'created_at'];

        // Check header completeness with flexible matching
        const missingHeaders = requiredFields.filter((rf) => !headers.some((h) => h === rf || h.includes(rf) || rf.includes(h)));
        if (missingHeaders.length > 0) {
          const err = `File thiếu các cột bắt buộc: ${missingHeaders.join(', ')}`;
          setFileError(err);
          if (onReportError) onReportError('Data Test Engine', 'Lỗi Form Upload Excel Test', err, 'high');
          return;
        }

        const parsedRows: TestRowItem[] = [];
        const validationErrors: string[] = [];

        for (let i = 1; i < lines.length; i++) {
          const rawRow = lines[i].trim();
          if (!rawRow) continue;

          // Split columns using auto-detected delimiter
          const cols = rawRow
            .split(delimiter)
            .map((c) => c.replace(/^["']|["']$/g, '').trim());

          if (cols.length < Math.min(headers.length, requiredFields.length)) continue;

          const rowData: any = {};
          headers.forEach((h, idx) => {
            const matchedKey = requiredFields.find((rf) => h === rf || h.includes(rf) || rf.includes(h)) || h;
            rowData[matchedKey] = cols[idx] || '';
          });

          // Validate row mandatory fields
          const emptyRequired = requiredFields.filter((rf) => !rowData[rf] || rowData[rf].trim() === '');
          if (emptyRequired.length > 0) {
            validationErrors.push(`Dòng ${i + 1}: Thiếu dữ liệu bắt buộc [${emptyRequired.join(', ')}]`);
            continue;
          }

          parsedRows.push({
            session_id: rowData['session_id'],
            customer_name: rowData['customer_name'],
            customer_phone: rowData['customer_phone'] || '',
            channel: (rowData['channel'] || 'CHAT').toUpperCase(),
            status: (rowData['status'] || 'RESOLVED').toUpperCase(),
            assigned_cs: rowData['assigned_cs'] || '',
            created_at: rowData['created_at'],
            rating: rowData['rating'] ? parseInt(rowData['rating'], 10) : 5,
            issue_category: rowData['issue_category'] || 'Dịch vụ chung',
            last_message: rowData['last_message'] || '',
          });
        }

        if (validationErrors.length > 0) {
          const firstErr = validationErrors[0];
          setFileError(`⚠️ Đã phát hiện lỗi trong file: ${firstErr} (Tổng lỗi: ${validationErrors.length})`);
          if (onReportError) {
            onReportError('Data Test Engine', 'Dữ liệu Excel Test Thiếu Trường Bắt Buộc', firstErr, 'high');
          }
          return;
        }

        setTestRows(parsedRows);
        setSuccessMsg(`✅ Đã đọc thành công ${parsedRows.length} dòng dữ liệu test hợp lệ từ file ${file.name}`);
      } catch (err: any) {
        setFileError(err.message || 'Lỗi đọc file Excel/CSV');
      }
    };
    reader.readAsText(file, 'UTF-8');
  };

  // Activate Test Report Mode
  const handleUpdateAndTest = () => {
    if (testRows.length === 0) {
      setFileError('Vui lòng chọn file Excel/CSV hợp lệ trước khi bấm Update & Test Report.');
      return;
    }

    sessionStorage.setItem('DD_TEST_REPORT_DATA', JSON.stringify(testRows));
    sessionStorage.setItem('DD_TEST_REPORT_ACTIVE', 'true');
    setIsTestActive(true);
    setSuccessMsg(`🚀 ĐÃ BẬT CÔNG TẮC TEST REPORT SUCCESS! Hệ thống Báo Cáo CX đã chuyển sang luồng test với ${testRows.length} bản ghi tạm thời.`);
  };

  // Deactivate Test Report Mode
  const handleCloseTest = () => {
    sessionStorage.removeItem('DD_TEST_REPORT_DATA');
    sessionStorage.removeItem('DD_TEST_REPORT_ACTIVE');
    setIsTestActive(false);
    setTestRows([]);
    setSuccessMsg('🔴 ĐÃ ĐÓNG TEST REPORT! Dữ liệu test tạm thời đã được xóa sạch. Hệ thống Báo Cáo CX đã ngắt luồng test và quay về CSDL chính.');
  };

  return (
    <div className="partner-wrapper">
      <div className="dashboard-control-bar">
        <div>
          <h2 style={{ fontSize: '18px', fontWeight: 700, color: '#fff', margin: 0, display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span>🧪</span> UPDATE DATA TEST (TEST DATA ENGINE — ĐẶC QUYỀN OWNER)
          </h2>
          <p style={{ fontSize: '12.5px', color: '#94a3b8', margin: '4px 0 0 0' }}>
            Nạp file Excel/CSV mô phỏng ca chat để tính toán thử nghiệm toàn bộ hệ thống Báo Cáo &amp; Phân Tích CX.
          </p>
        </div>

        {/* Action Button: Close Test if Active */}
        {isTestActive && (
          <button className="btn-filter-apply" onClick={handleCloseTest} style={{ background: '#ef4444', color: '#fff', fontWeight: 700 }}>
            <span>🔴 ĐÓNG TEST REPORT (NGẮT LUỒNG TEST)</span>
          </button>
        )}
      </div>

      {/* Test Status Banner */}
      <div
        style={{
          background: isTestActive ? 'rgba(168, 85, 247, 0.15)' : 'rgba(30, 41, 59, 0.6)',
          border: `1px solid ${isTestActive ? '#c084fc' : 'rgba(255, 255, 255, 0.1)'}`,
          borderRadius: '10px',
          padding: '14px 18px',
          marginBottom: '20px',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
          <span style={{ fontSize: '24px' }}>{isTestActive ? '🧪' : '💤'}</span>
          <div>
            <div style={{ fontSize: '14px', fontWeight: 700, color: isTestActive ? '#c084fc' : '#94a3b8' }}>
              TRẠNG THÁI LUỒNG BÁO CÁO: {isTestActive ? 'BẬT CHẾ ĐỘ TEST REPORT (Dùng data ảo tạm thời)' : 'CHẾ ĐỘ THỰC (GỌI API CSDL POSTGRESQL CHÍNH)'}
            </div>
            <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px' }}>
              {isTestActive
                ? `Đang chạy trên ${testRows.length} ca chat ảo. Dữ liệu KHÔNG BAO GIỜ lưu CSDL chính.`
                : 'Mọi báo cáo hiển thị đang truy vấn số liệu thực tế từ CSDL hệ thống.'}
            </div>
          </div>
        </div>

        <button className="btn-primary-purple" onClick={handleDownloadSample} style={{ fontSize: '12.5px', padding: '6px 12px' }}>
          <span>📥 Tải Form Mẫu Excel (.csv)</span>
        </button>
      </div>

      {/* Error & Success Messages */}
      {fileError && (
        <div style={{ background: 'rgba(239, 68, 68, 0.15)', border: '1px solid #ef4444', color: '#fca5a5', padding: '12px 16px', borderRadius: '8px', marginBottom: '16px', fontSize: '13px' }}>
          {fileError}
        </div>
      )}

      {successMsg && (
        <div style={{ background: 'rgba(34, 197, 94, 0.15)', border: '1px solid #22c55e', color: '#86efac', padding: '12px 16px', borderRadius: '8px', marginBottom: '16px', fontSize: '13px' }}>
          {successMsg}
        </div>
      )}

      {/* Excel Upload Form Card */}
      <div style={{ background: '#131a2b', border: '1px solid rgba(255, 255, 255, 0.1)', borderRadius: '12px', padding: '24px', marginBottom: '24px' }}>
        <h3 style={{ fontSize: '15px', fontWeight: 700, color: '#fff', marginBottom: '16px', display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span>📂</span> BƯỚC 1: NHẬP FILE DỮ LIỆU EXCEL / CSV TEST
        </h3>

        <div style={{ display: 'flex', alignItems: 'center', gap: '16px', flexWrap: 'wrap', marginBottom: '20px' }}>
          <label className="btn-filter-apply" style={{ cursor: 'pointer', background: '#3b82f6', color: '#fff', padding: '10px 18px', fontSize: '13px', display: 'inline-flex', alignItems: 'center', gap: '8px' }}>
            <span>📁 Chọn File Excel / CSV</span>
            <input type="file" accept=".csv, .xlsx, .xls" onChange={handleFileUpload} style={{ display: 'none' }} />
          </label>
          {selectedFileName && <span style={{ fontSize: '13px', color: '#38bdf8', fontWeight: 600 }}>📄 File đã chọn: {selectedFileName}</span>}
        </div>

        <div style={{ background: 'rgba(15, 23, 42, 0.6)', padding: '14px', borderRadius: '8px', border: '1px solid rgba(255, 255, 255, 0.05)', fontSize: '12.5px', color: '#94a3b8', lineHeight: '1.6' }}>
          <strong style={{ color: '#fbbf24' }}>⚠️ Các trường dữ liệu BẮT BUỘC trong file Excel:</strong>
          <ul style={{ margin: '6px 0 0 20px', padding: 0 }}>
            <li><code style={{ color: '#38bdf8' }}>session_id</code>: Mã phiên chat (Ví dụ: SESS-1001)</li>
            <li><code style={{ color: '#38bdf8' }}>customer_name</code>: Họ và tên khách hàng</li>
            <li><code style={{ color: '#38bdf8' }}>channel</code>: Kênh hỗ trợ (<code style={{ color: '#a855f7' }}>CHAT</code> hoặc <code style={{ color: '#a855f7' }}>CALL</code>)</li>
            <li><code style={{ color: '#38bdf8' }}>status</code>: Trạng thái (<code style={{ color: '#a855f7' }}>RESOLVED</code>, <code style={{ color: '#a855f7' }}>AI_ACTIVE</code>, <code style={{ color: '#a855f7' }}>NEEDS_HUMAN_CS</code>)</li>
            <li><code style={{ color: '#38bdf8' }}>created_at</code>: Ngày tạo (<code style={{ color: '#a855f7' }}>YYYY-MM-DD HH:mm:ss</code>)</li>
          </ul>
        </div>
      </div>

      {/* Preview Table of Uploaded Test Rows */}
      {testRows.length > 0 && (
        <div style={{ background: '#131a2b', border: '1px solid rgba(168, 85, 247, 0.3)', borderRadius: '12px', padding: '24px', marginBottom: '24px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px' }}>
            <h3 style={{ fontSize: '15px', fontWeight: 700, color: '#fff', margin: 0 }}>
              📊 BƯỚC 2: XEM TRƯỚC DỮ LIỆU TEST MÔ PHỎNG ({testRows.length} Ca Chat)
            </h3>

            <button className="btn-primary-purple" onClick={handleUpdateAndTest} style={{ padding: '10px 20px', fontSize: '13.5px', fontWeight: 700 }}>
              <span>🚀 UPDATE &amp; TEST REPORT</span>
            </button>
          </div>

          <div className="table-container" style={{ maxHeight: '350px', overflowY: 'auto' }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>STT</th>
                  <th>Session ID</th>
                  <th>Khách Hàng</th>
                  <th>Kênh</th>
                  <th>Trạng Thái</th>
                  <th>CSKH Phụ Trách</th>
                  <th>Điểm CSAT</th>
                  <th>Phân Loại Sự Cố</th>
                  <th>Ngày Tạo</th>
                </tr>
              </thead>
              <tbody>
                {testRows.map((r, i) => (
                  <tr key={i}>
                    <td>{i + 1}</td>
                    <td><span style={{ color: '#c084fc', fontWeight: 600 }}>{r.session_id}</span></td>
                    <td><strong>{r.customer_name}</strong></td>
                    <td><span className={`status-pill ${r.channel === 'CALL' ? 'pill-green' : 'pill-blue'}`}>{r.channel}</span></td>
                    <td><span className="status-pill pill-blue">{r.status}</span></td>
                    <td>{r.assigned_cs || 'AI Engine'}</td>
                    <td>⭐ {r.rating || 5}</td>
                    <td>{r.issue_category || '-'}</td>
                    <td>{r.created_at}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};
