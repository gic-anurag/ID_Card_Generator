package service

import (
	"context"
	"errors"
	"fmt"
	"idGenerator/pojo"
	"io"
	"log"
	"mime/multipart"
	"os"
	"strings"
	"time"

	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Connection struct {
	Server     string
	Database   string
	Collection string
}

const maxUploadSize = 10 * 1024 * 1024 // 10 mb
const dir = "data/download/"
const downloadDir = "download/"

var Collection *mongo.Collection
var ctx = context.TODO()
var idGenerated int

func (e *Connection) Connect() {
	clientOptions := options.Client().ApplyURI(e.Server)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	err = license.SetMeteredKey("301d8f2e0d0c5d045070142329639ac70eda204a4ad3039482d1bd6d023a2f9a")
	if err != nil {
		log.Fatal(err)
	}

	Collection = client.Database(e.Database).Collection(e.Collection)

}

func (e *Connection) CreateIdAndStore(dataBody pojo.Request, files []*multipart.FileHeader) (string, error) {
	bool, err := validateByNameAndDob(dataBody)
	if err != nil {
		return "", err
	}
	if !bool {
		return "", errors.New("User already present")
	}
	file, err := uploadFile(files)
	data, err := fetchAllData()
	if err != nil {
		return "", err
	}
	var id int64
	fmt.Println("Lowest ID:", data[0].IdNo)
	fmt.Println("Highest ID", data[len(data)-1].IdNo)
	if len(data) != 0 {
		id = data[len(data)-1].IdNo + 1
	} else {
		id = 1
	}
	saveData, err := SetValueInModel(dataBody, id, file)
	if err != nil {
		return "", errors.New("Unable to parse date")
	}
	if _, err := Collection.InsertOne(ctx, saveData); err != nil {
		log.Println(err)
		return "", errors.New("Unable to store data")
	}

	return "Generated Id : " + fmt.Sprintf("%v", id), nil
}

func validateByNameAndDob(reqbody pojo.Request) (bool, error) {
	dobStr := reqbody.DOB
	dob, err := convertDate(dobStr)
	if err != nil {
		return false, err
	}
	fmt.Println(dob)
	var result []*pojo.Idcard
	data, err := Collection.Find(ctx, bson.D{{"name", reqbody.Name}, {"dob", dob}, {"active", true}})
	if err != nil {
		return false, err
	}
	result, err = convertDbResultIntoStruct(data)
	if err != nil {
		return false, err
	}
	if len(result) == 0 {
		return true, err
	}
	return false, err
}

func convertDate(dateStr string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		log.Println(err)
		return date, err
	}
	return date, nil
}

func convertDbResultIntoStruct(fetchDataCursor *mongo.Cursor) ([]*pojo.Idcard, error) {
	var finaldata []*pojo.Idcard
	for fetchDataCursor.Next(ctx) {
		var data pojo.Idcard
		err := fetchDataCursor.Decode(&data)
		if err != nil {
			return finaldata, err
		}
		finaldata = append(finaldata, &data)
	}
	return finaldata, nil
}

func fetchDataByActive() ([]*pojo.Idcard, error) {
	var result []*pojo.Idcard
	filter := bson.D{}
	sorting := options.Find().SetSort(bson.D{{"id", -1}})
	data, err := Collection.Find(ctx, filter, sorting)
	if err != nil {
		return result, err
	}
	result, err = convertDbResultIntoStruct(data)
	if err != nil {
		return result, err
	}

	return result, err
}

func SetValueInModel(req pojo.Request, id int64, file []string) (pojo.Idcard, error) {
	var data pojo.Idcard
	joiningDate, err := convertDate(req.JoiningDate)
	if err != nil {
		log.Println(err)
		return data, err
	}
	dob, err := convertDate(req.DOB)
	if err != nil {
		log.Println(err)
		return data, err
	}
	data.JoiningDate = joiningDate
	data.DOB = dob
	data.CreatedDate = time.Now()
	data.Name = req.Name
	data.Age = req.Age
	data.Designation = req.Designation
	data.BloodGroup = req.BloodGroup
	data.Active = true
	data.IdNo = id
	data.FileLocation = file
	return data, nil
}

func (e *Connection) FetchAllData() ([]*pojo.Idcard, error) {
	var finaldata []*pojo.Idcard
	fetchDataCursor, err := Collection.Find(ctx, bson.D{primitive.E{Key: "active", Value: true}})
	if err != nil {
		return finaldata, err
	}
	finaldata, err = convertDbResultIntoStruct(fetchDataCursor)
	if err != nil {
		return finaldata, err
	}
	if finaldata == nil {
		return finaldata, errors.New("Either Db is empty or all data is deactivated")
	}
	return finaldata, err
}

func (e *Connection) FetchDataById(idStr string) ([]*pojo.Idcard, error) {
	var finaldata []*pojo.Idcard
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return finaldata, err
	}
	fetchDataCursor, err := Collection.Find(ctx, bson.D{{"_id", id}, {"active", true}})
	if err != nil {
		return finaldata, err
	}

	finaldata, err = convertDbResultIntoStruct(fetchDataCursor)
	if err != nil {
		return finaldata, err
	}

	if len(finaldata) == 0 {
		return finaldata, errors.New("Data not present in db given by Id or it is deactivated")
	}
	str, err := writeDataIntoPDFTable(finaldata)
	fmt.Println(str)
	return finaldata, err
}

func (e *Connection) UpdateDataById(idStr string, reqData pojo.Request) (bson.M, error) {
	var updatedDocument bson.M
	idk, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return updatedDocument, err
	}

	filter := bson.D{
		{"$and",
			bson.A{
				bson.D{{"_id", idk}},
				bson.D{{"active", true}},
			},
		},
	}
	UpdateQuery := bson.D{}
	if reqData.Name != "" {
		UpdateQuery = append(UpdateQuery, primitive.E{Key: "name", Value: reqData.Name})
	}
	if reqData.Age != 0 {
		UpdateQuery = append(UpdateQuery, primitive.E{Key: "age", Value: reqData.Age})
	}
	if reqData.BloodGroup != "" {
		UpdateQuery = append(UpdateQuery, primitive.E{Key: "blood_group", Value: reqData.BloodGroup})
	}
	if reqData.Designation != "" {
		UpdateQuery = append(UpdateQuery, primitive.E{Key: "designation", Value: reqData.Designation})
	}
	if reqData.DOB != "" {
		dob, err := convertDate(reqData.DOB)
		if err != nil {
			log.Println(err)
			return updatedDocument, err
		}
		UpdateQuery = append(UpdateQuery, primitive.E{Key: "dob", Value: dob})
	}
	if reqData.JoiningDate != "" {
		joiningDate, err := convertDate(reqData.JoiningDate)
		if err != nil {
			log.Println(err)
			return updatedDocument, err
		}
		UpdateQuery = append(UpdateQuery, primitive.E{Key: "dob", Value: joiningDate})
	}
	update := bson.D{{"$set", UpdateQuery}}

	r := Collection.FindOneAndUpdate(ctx, filter, update).Decode(&updatedDocument)
	if r != nil {
		return updatedDocument, r
	}
	fmt.Println(updatedDocument)
	if updatedDocument == nil {
		return updatedDocument, errors.New("Data not present in db given by Id or it is deactivated")
	}
	return updatedDocument, err
}

func (e *Connection) DeleteById(idStr string) (string, error) {
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return "", err
	}
	filter := bson.D{primitive.E{Key: "_id", Value: id}}
	update := bson.D{{"$set", bson.D{primitive.E{Key: "active", Value: false}}}}
	Collection.FindOneAndUpdate(ctx, filter, update)
	return "Documents Deactivated Successfully", err
}

func uploadFile(files []*multipart.FileHeader) ([]string, error) {
	var fileNames []string

	for _, fileHeader := range files {
		fileName := fileHeader.Filename
		fileNames = append(fileNames, dir+fileName)
		if fileHeader.Size > maxUploadSize {
			return fileNames, errors.New("The uploaded image is too big: %s. Please use an image less than 1MB in size: " + fileHeader.Filename)
		}

		file, err := fileHeader.Open()
		if err != nil {
			return fileNames, err
		}

		defer file.Close()

		buff := make([]byte, 512)
		_, err = file.Read(buff)
		if err != nil {
			return fileNames, err
		}

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return fileNames, err
		}

		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return fileNames, err
		}

		f, err := os.Create(dir + fileHeader.Filename)
		if err != nil {
			return fileNames, err
		}

		defer f.Close()

		_, err = io.Copy(f, file)
		if err != nil {
			return fileNames, err
		}
	}

	return fileNames, nil
}

func fetchAllData() ([]*pojo.Idcard, error) {
	var result []*pojo.Idcard
	filter := bson.D{}
	sorting := options.Find().SetSort(bson.D{{"id", -1}})
	data, err := Collection.Find(ctx, filter, sorting)
	if err != nil {
		return result, err
	}
	result, err = convertDbResultIntoStruct(data)
	if err != nil {
		return result, err
	}

	return result, err
}

func writeDataIntoPDFTable(data []*pojo.Idcard) (string, error) {
	str := "File Download Successfully :" + downloadDir + fmt.Sprintf("%v", data[0].IdNo) + ".pdf"
	c := creator.New()
	c.SetPageMargins(20, 20, 20, 20)

	font, err := model.NewStandard14Font(model.HelveticaName)
	if err != nil {
		return "", err
	}

	fontBold, err := model.NewStandard14Font(model.HelveticaBoldName)
	if err != nil {
		return "", err
	}

	if err := basicUsage(c, font, fontBold, data); err != nil {
		return "", err
	}
	err = os.MkdirAll(downloadDir, os.ModePerm)
	if err != nil {
		return "", err
	}
	err = c.WriteToFile(downloadDir + data[0].Name + fmt.Sprintf("%v", data[0].IdNo) + ".pdf")
	if err != nil {
		return "", err
	}
	return str, nil
}

func basicUsage(c *creator.Creator, font, fontBold *model.PdfFont, data []*pojo.Idcard) error {
	// Create chapter.
	ch := c.NewChapter("Id Card")
	ch.SetMargins(100, 0, 50, 0)
	ch.GetHeading().SetFont(font)
	ch.GetHeading().SetFontSize(20)
	ch.GetHeading().SetColor(creator.ColorRGBFrom8bit(72, 86, 95))

	contentAlignH(c, ch, font, fontBold, data)

	if err := c.Draw(ch); err != nil {
		return err
	}
	return nil
}

func contentAlignH(c *creator.Creator, ch *creator.Chapter, font, fontBold *model.PdfFont, data []*pojo.Idcard) {

	normalFontColorGreen := creator.ColorRGBFrom8bit(4, 79, 3)
	normalFontSize := 10.0
	for i := range data {

		img, err := c.NewImageFromFile(data[i].FileLocation[0])
		if err != nil {
			log.Println(err)
		}
		img.ScaleToHeight(50)
		img.SetMargins(120, 0, 20, 0)
		ch.Add(img)

		x := c.NewParagraph("ID" + " :     " + fmt.Sprintf("%v", data[i].IdNo))
		x.SetFont(font)
		x.SetFontSize(normalFontSize)
		x.SetColor(normalFontColorGreen)
		x.SetMargins(100, 0, 10, 0)
		ch.Add(x)
		y := c.NewParagraph("Name" + " :     " + data[i].Name)
		y.SetFont(font)
		y.SetFontSize(normalFontSize)
		y.SetColor(normalFontColorGreen)
		y.SetMargins(100, 0, 10, 0)
		ch.Add(y)
		z := c.NewParagraph("DOB" + " :     " + strings.Trim(data[i].DOB.String(), " 00:00:00 +0000 UTC"))
		z.SetFont(font)
		z.SetFontSize(normalFontSize)
		z.SetColor(normalFontColorGreen)
		z.SetMargins(100, 0, 10, 0)
		ch.Add(z)
		b := c.NewParagraph("Designation" + ":     " + data[i].Designation)
		b.SetFont(font)
		b.SetFontSize(normalFontSize)
		b.SetColor(normalFontColorGreen)
		b.SetMargins(100, 0, 10, 0)
		ch.Add(b)
		a := c.NewParagraph("Blood Group" + ":     " + data[i].BloodGroup)
		a.SetFont(font)
		a.SetFontSize(normalFontSize)
		a.SetColor(normalFontColorGreen)
		a.SetMargins(100, 0, 10, 0)
		//		a.SetLineHeight(2)
		ch.Add(a)
	}
}
