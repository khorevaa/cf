
local lfs = require "lfs"
local miniz = require "miniz"
local cf = require "cf"

local END = "\xFF\xFF\xFF\x7F"

local function write(path, data)
    local file = assert(io.open(path, "wb"))
    file:write(data)
    file:close()
end

local function UnpackTo(path, rd)

    local Image = cf.ReadImage(rd)
    local ret, res, dir

    for _, id, body, packed in Image.Rows() do
        if packed then
            ret, res = pcall(miniz.inflate, body, 0)
            if ret ~= true then -- xml?
                write(path .. id, body)
            else
                if res:sub(1, 4) == END then
                    dir = path .. id .. "/"
                    lfs.mkdir(dir)
                    UnpackTo(dir, cf.StringReader(res))
                else
                    write(path .. id, res)
                end
            end
        else
            write(path .. id, body)
        end
    end

end

local file = assert(io.open(arg[1] or "C:/temp/RU/1Cv8.cf", "rb"))
local dir = arg[2] or "C:/temp/RU/1Cv8_cf/"
dir = dir:sub(-1) == '/' and dir or dir..'/'
lfs.mkdir(dir)
-- ProFi = require 'ProFi'
-- ProFi:start()
UnpackTo(dir, cf.FileReader(file))
-- ProFi:stop()
-- ProFi:writeReport( 'ProfilingReport.txt' )