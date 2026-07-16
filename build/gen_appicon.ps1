Add-Type -AssemblyName System.Drawing
$size = 1024
$out  = 'D:\development\code\Go\port_lite\build\appicon.png'

$bmp = New-Object System.Drawing.Bitmap($size, $size)
$g   = [System.Drawing.Graphics]::FromImage($bmp)
$g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$g.Clear([System.Drawing.Color]::Transparent)

# teal gradient square background
$rect  = [System.Drawing.Rectangle]::new(0, 0, $size, $size)
$top   = [System.Drawing.ColorTranslator]::FromHtml('#2DD4BF')
$bot   = [System.Drawing.ColorTranslator]::FromHtml('#0D9488')
$brush = [System.Drawing.Drawing2D.LinearGradientBrush]::new($rect, $top, $bot, [System.Drawing.Drawing2D.LinearGradientMode]::Vertical)
$g.FillRectangle($brush, $rect)

# white terminal glyph: ">" chevron + "_" underscore
$white = [System.Drawing.Color]::FromArgb(255, 255, 255, 255)
$pen = [System.Drawing.Pen]::new($white, 80.0)
$pen.StartCap = [System.Drawing.Drawing2D.LineCap]::Round
$pen.EndCap   = [System.Drawing.Drawing2D.LineCap]::Round
$pen.LineJoin = [System.Drawing.Drawing2D.LineJoin]::Round

$p1 = [System.Drawing.PointF]::new(400.0, 400.0)
$p2 = [System.Drawing.PointF]::new(585.0, 512.0)
$p3 = [System.Drawing.PointF]::new(400.0, 624.0)
$g.DrawLines($pen, @($p1, $p2, $p3))
$g.DrawLine($pen, 625.0, 624.0, 810.0, 624.0)

$bmp.Save($out, [System.Drawing.Imaging.ImageFormat]::Png)
$g.Dispose(); $bmp.Dispose()
Write-Output ('icon -> ' + $out)
