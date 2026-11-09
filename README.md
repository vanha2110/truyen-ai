# ConvertFix

Ứng dụng Windows biên tập truyện convert bằng Ollama. Go + SQLite giữ dữ liệu và hàng đợi; React/TypeScript chạy trong Wails. Người dùng **tự nhập nguồn truyện**, không có danh sách website đóng cứng.

## Dùng ứng dụng

1. Mở `build/bin/ConvertFix-0.1.0.exe`, hoặc dùng bộ cài trong cùng thư mục.
2. Mở Ollama. Vào **Cài đặt**, kiểm tra kết nối, chọn model **Biên tập**, rồi **Lưu cài đặt**. Model trích xuất và tóm tắt là tùy chọn.
3. Vào **Thêm truyện**: nhập URL trang mục lục và xem trước, hoặc nhập tệp/văn bản UTF-8. Với website chưa được nhận diện tốt, cấu hình selector CSS cho mục lục và nội dung chương.
4. Mở truyện, chọn khoảng chương, bấm **Tải và biên tập**. Theo dõi tại **Hàng đợi**. Job thiếu model hoặc nguồn lỗi sẽ dừng với lý do rõ ràng; Ollama mất kết nối sẽ tự thử lại.
5. Đọc trong **Reader**, chọn bản convert/biên tập/song song. Chunk lỗi giữ nguyên bản gốc và đánh dấu **Cần xem lại**. Sửa tay cần tạm dừng job liên quan; bản sửa không bị tự động ghi đè.
6. Thêm thuật ngữ trước khi chạy các chương tiếp theo. Khi thay tên trong chương đã có, dùng **Xem trước thay tên toàn truyện** rồi áp dụng; Reader có hoàn tác.

Đóng cửa sổ khi đang biên tập sẽ cho chọn thoát an toàn hoặc thu xuống khay. Menu khay có mở cửa sổ và thoát. Hẹn giờ và biên tập trước chỉ hoạt động khi tiến trình đang chạy. Khởi động cùng Windows và thông báo là tùy chọn, mặc định tắt.

## Dữ liệu

Mặc định: `%AppData%\ConvertFix` gồm `convertfix.db`, `backups`, `logs`, `updates`. Biến môi trường `CONVERTFIX_DATA` cho phép chọn thư mục khác trước khi mở app. CLI và desktop dùng chung core và cơ chế khóa thư mục, nên không mở đồng thời trên cùng DB.

- SQLite WAL, synchronous FULL, foreign keys, migration có phiên bản.
- Chia chunk một lần, lưu raw riêng, transaction khi hoàn tất chunk và ghép chương cuối.
- Mở lại sau crash: chunk đang xử lý về pending, tăng crash count; crash cùng chunk hai lần thì giữ raw và đánh dấu review.
- Pause do người dùng được giữ qua restart. Thoát bình thường không tăng crash count.
- Sau mỗi job: backup bằng `VACUUM INTO`, giữ 5 bản gần nhất; backup trước migration và tải bản cập nhật.
- Để phục hồi thủ công: đóng toàn bộ ConvertFix; giữ riêng **cả bộ DB hiện tại và các file WAL/SHM** nếu có; đưa một file `.db` từ `backups` sang thư mục dữ liệu mới với tên `convertfix.db`, rồi chạy với `CONVERTFIX_DATA` trỏ vào thư mục mới. Không chép đè DB đang mở.

Các thư mục `.cache/ui-smoke-data`, `.cache/smoke-data`, `.cache/pipeline-smoke` là dữ liệu kiểm thử riêng, không phải thư viện mặc định.

## Build và phát triển

Yêu cầu **Go global 1.26+**, Node.js 20.19+/22+, WebView2; NSIS chỉ cần khi tạo installer. Không dùng Go toolchain riêng trong dự án. `scripts/build.ps1` tìm `go` trên PATH hoặc `C:\Program Files\Go\bin` và đặt `GOTOOLCHAIN=local`.

```powershell
.\scripts\dev.ps1
.\scripts\build.ps1
.\scripts\build.ps1 -Installer
```

Script chỉ đặt cache dependency/Wails tại `.cache`, không tải hay cài Go SDK. Wails CLI được khóa ở `v2.17.1`; dependency frontend dùng `package-lock.json`. Installer có phạm vi người dùng, dữ liệu nằm ngoài thư mục cài đặt.

Kiểm tra riêng:

```powershell
npm ci --prefix frontend
npm run build --prefix frontend
go test ./... -count=1
go vet ./...
```

Frontend bật `strict`, `strictNullChecks`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`; route và trạng thái dùng union, không dùng `any` hay ép kiểu `as` trong mã ứng dụng.

## CLI

```powershell
# Chạy với DB riêng nếu desktop đang mở
$env:CONVERTFIX_DATA = Join-Path $PWD '.cache\my-library'
.\build\bin\convertfix-cli.exe models
.\build\bin\convertfix-cli.exe bench 'ten-model' testdata/convert/sample-01.txt
.\build\bin\convertfix-cli.exe edit input.txt --model 'ten-model'
.\build\bin\convertfix-cli.exe import input.txt
.\build\bin\convertfix-cli.exe add 'https://nguon-cua-ban/truyen' source.json
.\build\bin\convertfix-cli.exe fetch --novel 1 --from 1 --to 10
.\build\bin\convertfix-cli.exe process --novel 1 --from 1 --to 10 --model 'ten-model'
.\build\bin\convertfix-cli.exe run
.\build\bin\convertfix-cli.exe jobs
.\build\bin\convertfix-cli.exe backup
```

Ví dụ `source.json` (selector do người dùng kiểm tra trên website cụ thể):

```json
{
  "title": "h1",
  "author": "[itemprop=author]",
  "description": "[itemprop=description]",
  "chapterLinks": ".chapter-list a",
  "content": ".chapter-content",
  "next": "a.next-page",
  "remove": "script,style,nav,footer,header,aside,iframe,form,button,.ads"
}
```

Crawler chỉ truy cập HTTP(S) công khai, kiểm tra DNS/redirect để tránh truy cập mạng nội bộ, đọc robots.txt, giãn request tối thiểu 1,5 giây, dừng khi gặp 403/429. Mục lục tự duyệt liên kết phân trang HTML, trang số, select và mẫu AJAX GET trả HTML trong JSON `data` (đã kiểm tra wikidichvn.com). Không thực thi JavaScript tải từ website. Nguồn dùng kiểu JavaScript khác hoặc chống bot có thể cần cấu hình riêng hoặc nhập văn bản. Selector và mục lục được lưu theo từng truyện. Nếu nguồn có nhiều URL khác nhau mang cùng số chương, app giữ tất cả theo thứ tự mục lục, cảnh báo và dùng số thứ tự liên tục; tiêu đề/URL gốc được giữ nguyên.

## Glossary và model

Glossary hỗ trợ global/riêng truyện, khóa, ưu tiên, biến thể, CSV/JSON. So khớp Aho–Corasick trên rune NFC, kiểm tra ranh giới từ tiếng Việt, ưu tiên khớp dài nhất và thay một lượt. CSV dùng header `source,target,kind,variants`, các biến thể phân cách bằng `|`. Import không ghi đè mục khóa và rollback toàn bộ khi có lỗi.

Model catalog lấy từ `/api/tags`, `/api/show`, `/api/ps`; model không có `completion` không được chọn biên tập. Request luôn có `num_ctx`, `num_predict`; thinking chỉ gửi khi model hỗ trợ. Job lưu snapshot cấu hình ở chương. Model/preset riêng theo truyện được tạo từ cấu hình đang lưu trong trang Cài đặt; có thể trở về cấu hình chung.

Prompt dùng glossary liên quan, quy tắc xưng hô, tối đa 3 few-shot được duyệt và ngữ cảnh dựng lại từ DB. Validation kiểm tra số/thứ tự đoạn, tỷ lệ từ, ký tự Hán, lời dẫn/thinking, tên chuẩn, lặp và output bị cắt. Không có validator tự động nào ở đây chứng minh được tính đúng nghĩa hoặc không bịa tình tiết; cần người đọc chấm lại.

## Cập nhật

Đã có kiểm tra manifest HTTPS, chữ ký Ed25519, SHA256 của installer, tải có giới hạn dung lượng và sao lưu trước cập nhật. Cần tự cung cấp kênh phát hành và public key; app không trỏ đến một server giả định. Bản này tải và xác minh installer, sau đó người dùng đóng app và chạy installer. Chưa tự thay thế binary khi đang chạy. Xem [quy trình phát hành](docs/RELEASING.md).

## Trạng thái nghiệm thu

Đã kiểm tra unit/integration, kill một tiến trình khi transaction chưa commit, pipeline Ollama thật 5 chương tổng hợp và mở giao diện Windows. Chưa nghiệm thu tải 100 chương từ một website cụ thể, chạy qua đêm 50–100 chương, mất điện VM hay cài sạch trên máy Windows khác. Không coi bản 0.1.0 là đã đạt toàn bộ ngưỡng chất lượng trong kế hoạch. Xem [kiểm chứng và phần còn lại](docs/IMPLEMENTATION.md).

Tài liệu nền tảng: [Wails v2](https://v2.wails.io/docs/gettingstarted/installation/), [Ollama API](https://docs.ollama.com/api), [NSIS](https://nsis.sourceforge.io/Download).

## Lỗi kết nối nguồn và robots.txt

App phân biệt quy tắc `Disallow` với lỗi HTTP hoặc lỗi xác thực HTTPS:

- robots.txt trả 404/410 hoặc HTTP 4xx khác (trừ 429): coi là không có robots khả dụng; tiếp tục kiểm tra trang truyện, vẫn giới hạn tốc độ. HTTP 401/403 ở **trang truyện** không được bỏ qua.
- robots.txt trả 429: dừng vì rate limit; 5xx/lỗi mạng: tạm dừng thay vì suy đoán cho phép.
- robots hợp lệ có `Disallow`: giữ nguyên quy tắc. Cache tối đa 24 giờ; redirect robots có thể đến host công khai khác nhưng không hạ HTTPS xuống HTTP.
- Lỗi TLS có thông báo riêng; app không dùng `InsecureSkipVerify` và không tự tin cậy chứng chỉ lạ. Lỗi chứng chỉ không bị queue tự retry vô hạn.

Ví dụ đã tái hiện với `metruyenchu.com.vn` trên môi trường kiểm tra: kết nối nhận chứng chỉ có issuer Fortinet và thất bại `x509: certificate signed by unknown authority`; PowerShell cũng báo `PartialChain`. Chưa đọc được robots hay mục lục. Đây là điều kiện của chuỗi chứng chỉ trên kết nối hiện tại, không phải kết luận rằng website cấm crawl. Cần quản trị mạng kiểm tra/cấp CA hợp lệ hoặc dùng nguồn đang kết nối được; bỏ bước robots không sửa được lỗi TLS.

Hành vi robots HTTP tham chiếu [RFC 9309 §2.3.1](https://www.rfc-editor.org/rfc/rfc9309.html#section-2.3.1).