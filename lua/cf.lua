local END  = 0x7FFFFFFF
local BOM  = "\xEF\xBB\xBF"
local CRLF = "\x0D\x0A"
local ImageHeaderLen = 4 + 4 + 4 + 4     -- uint32 + uint32 + uint32 + uint32
local PageHeaderLen  = 2 + 9 + 9 + 9 + 2 -- CRLF + hex8byte_ + hex8byte_ + hex8byte_ + CRLF
local RowHeaderLen   = 8 + 8 + 4         -- datetime + datetime + attr
local Unknown4       = 4

local null = setmetatable({}, {
    __tostring = function ()
        return "null"
    end
})

local Module = {}

local Reader = {}
Reader.__index = Reader

local FileReader = setmetatable({}, Reader)
FileReader.__index = FileReader
FileReader.file = null
FileReader.fpos = 0
FileReader.flen = 0
FileReader.offset = 0
FileReader.buflen = 0
FileReader.buffer = null

function Module.FileReader(file, buflen)
    local this = setmetatable({}, FileReader)
    this.file   = file
    this.offset = 0
    this.fpos   = 0
    this.flen   = file:seek("end")
    this.buflen = buflen or 256 * 1024
    file:seek("set")
    if this.flen < this.buflen then
        this.buflen = this.flen
    end
    this.buffer = file:read(this.buflen)
    return this
end

function FileReader:Seek(newpos)
    assert(newpos <= self.flen)
    self.offset = newpos % self.buflen
    newpos = newpos - self.offset
    if self.fpos ~= newpos then
        self.fpos = self.file:seek("set", newpos);
        self.buffer = self.file:read(math.min(self.buflen, self.flen - newpos))
    end
end

function FileReader:Read(len)
    local t = {}
    local tail = self.buflen - self.offset
    if len > tail then
        t[#t+1] = self.buffer:sub(self.offset + 1, self.buflen)
        self:Seek(self.fpos + self.buflen)
        len = len - tail
    end
    while len >= self.buflen do
        t[#t+1] = self.buffer
        self:Seek(self.fpos + self.buflen)
        len = len - self.buflen
    end
    if self.buffer then
        local newoffset = self.offset + len
        t[#t+1] = self.buffer:sub(self.offset + 1, newoffset)
        self.offset = newoffset
    end
    return table.concat(t)
end

local StringReader = setmetatable({}, Reader)
StringReader.__index = StringReader
StringReader.src = null
StringReader.len = 0
StringReader.pos = 0

function Module.StringReader(src)
    local this = setmetatable({}, StringReader)
    this.src = src
    this.len = #src
    this.pos = 0
    return this
end

function StringReader:Seek(newpos)
    assert(newpos <= self.len)
    self.pos = newpos
end

function StringReader:Read(len)
    local start = self.pos + 1
    self.pos = self.pos + len
    if self.pos <= self.len then
        return self.src:sub(start, self.pos)
    else
        return ""
    end
end

local function getUInt32(b0, b1, b2, b3)
    return b3 * 256^3 + b2 * 256^2 + b1 * 256 + b0
end

function Reader:ReadImageHeader()
    local s = self:Read(ImageHeaderLen)
    return {
        PageSize  = getUInt32(s:byte(5,  8)),
        Revision  = getUInt32(s:byte(9, 12)),
        Unknown   = getUInt32(s:byte(13, 16))
    }
end

function Reader:ReadPageHeader()
    local s = self:Read(PageHeaderLen)
    assert(s:sub( 1,  2) == CRLF)
    assert(s:sub(30, 31) == CRLF)
    return {
        FullSize = tonumber(s:sub( 3, 10), 16),
        PageSize = tonumber(s:sub(12, 19), 16),
        NextPage = tonumber(s:sub(21, 28), 16)
    }
end

function Reader:ReadRowHeader()
    local pageHeader = self:ReadPageHeader()
    assert(pageHeader.NextPage == END)
    local s = self:Read(RowHeaderLen)
    return {
        Creation   = s:sub(1,  8),
        Modified   = s:sub(9, 16),
        Attributes = getUInt32(s:byte(17, 20)),
        ID = self:Read(pageHeader.FullSize - RowHeaderLen - Unknown4)
    }
end

function Reader:ReadRowBody()
    local pageHeader = self:ReadPageHeader()
    local fullSize = pageHeader.FullSize
    local size = math.min(fullSize, pageHeader.PageSize)
    local t = {self:Read(size)}
    fullSize = fullSize - size
    while pageHeader.NextPage ~= END do
        self:Seek(pageHeader.NextPage)
        pageHeader = self:ReadPageHeader()
        size = math.min(fullSize, pageHeader.PageSize)
        t[#t+1] = self:Read(size)
        fullSize = fullSize - size
    end
    assert(fullSize == 0)
    return table.concat(t)
end

function Reader:ReadPointers()
    local t = {}
	local pageHeader = self:ReadPageHeader()
	local fullSize = pageHeader.FullSize
	local readSize = 0
    local size, data
    while pageHeader.NextPage ~= END do
        size = pageHeader.PageSize
        readSize = readSize + size
        data = self:Read(size)
        for i = 1, size, 4 do
			t[#t+1] = getUInt32(data:byte(i,  i + 3))
        end
		self:Seek(pageHeader.NextPage)
		pageHeader = self:ReadPageHeader()
    end
    size = fullSize - readSize
    data = self:Read(size)
    for i = 1, size, 4 do
        t[#t+1] = getUInt32(data:byte(i,  i + 3))
    end
    t.rd = self
	return t
end

local function iter(t, i)
    local id, body, header
    local pos = t[i]
    if pos == nil then
        return nil
    end
    assert(t[i+2] == END)
    t.rd:Seek(pos)
    header = t.rd:ReadRowHeader()
    id = string.gsub(header.ID, "%z", "")
    t.rd:Seek(t[i+1])
    body = t.rd:ReadRowBody()
    i = i + 3
    return i, id, body, body:sub(1, 3) ~= BOM, header
end

function Module.ReadImage(rd)
    local header = rd:ReadImageHeader()
    local pointers = rd:ReadPointers()
    return {
        Header = header,
        Pointers = pointers,
        Rows = function ()
            return iter, pointers, 1
        end,
        List = function ()
            local t = {}
            local ID
            for i = 1, #pointers, 3 do
                rd:Seek(pointers[i])
                ID = string.gsub(rd:ReadRowHeader().ID, "%z", "")
                t[ID] = pointers[i+1]
            end
            return t
        end
    }
end

function Module.Parse(s, beg)
    beg = beg or 1
    local pos = s:find('{', beg, true)
    if not pos then return nil end
    local qt, lb, rb = ('"{}'):byte(1,3)
    local skip, b = false, 0
    local function parse()
        beg = pos + 1
        pos = s:find('["{},]', pos + 1)
        local tree = {}
        while pos do
            b = s:byte(pos)
            if b == qt then
                skip = not skip
            elseif not skip then
                if b == lb then
                    tree[#tree+1] = assert(parse())
                    pos = pos + 1
                    beg = pos + 1
                elseif b == rb then
                    if beg < pos then
                        tree[#tree+1] = s:sub(beg, pos - 1)
                    end
                    return tree
                else -- ','
                    tree[#tree+1] = s:sub(beg, pos - 1)
                    beg = pos + 1
                end
            end
            pos = s:find('["{},]', pos + 1)
        end
        return {}
    end
    return parse()
end

return Module